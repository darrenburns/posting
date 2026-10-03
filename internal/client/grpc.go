package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	_ "google.golang.org/genproto/googleapis/rpc/errdetails" // Decodes the common status details.
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/darrenburns/posting/internal/model"
)

// describeDialTimeout bounds how long discovery waits to connect, whatever
// the request's own timeout, so the method list doesn't hang on a server
// that isn't there.
const describeDialTimeout = 3 * time.Second

// GRPC sends gRPC calls and describes gRPC schemas. Every Send and Describe
// dials a connection of its own and closes it before returning, so nothing
// outlives a call.
type GRPC struct {
	UserAgent string
	TLS       TLSSettings
	// Root is the directory proto paths are relative to: the collection's.
	Root string

	// maxResponse caps the bytes of messages kept from one call.
	maxResponse int

	protos protoCache
}

// NewGRPC returns a client that identifies itself as userAgent and finds
// proto files relative to root.
func NewGRPC(userAgent string, settings TLSSettings, root string) *GRPC {
	return &GRPC{UserAgent: userAgent, TLS: settings, Root: root, maxResponse: maxBodyBytes}
}

// grpcCall is a resolved gRPC request, checked as far as it can be without
// its schema or its server.
type grpcCall struct {
	req     model.Request
	payload model.GRPC
	md      metadata.MD
	// userAgent is the request's user-agent metadata, which grpc-go would
	// replace with its own, so it goes to the dialer instead. "" is unset.
	userAgent string
	timeout   float64
	lookup    func(string) (string, bool)
	// ctx carries the call's metadata and, once started, its timeout. An
	// open stream's loses the timeout once it connects.
	ctx context.Context
}

func prepare(call Call) (*grpcCall, error) {
	req, err := model.Resolve(call.Request, call.Lookup)
	if err != nil {
		return nil, err
	}
	payload, ok := req.Payload.(model.GRPC)
	if !ok {
		return nil, fmt.Errorf("%s requests aren't sent over gRPC", req.Kind().Label)
	}
	md, err := outgoingMetadata(req.Headers, req.Auth)
	if err != nil {
		return nil, err
	}
	userAgent := strings.Join(md.Get("user-agent"), " ")
	md.Delete("user-agent")
	timeout := req.Options.TimeoutSeconds
	if timeout <= 0 {
		timeout = model.DefaultOptions().TimeoutSeconds
	}
	return &grpcCall{req: req, payload: payload, md: md, userAgent: userAgent, timeout: timeout, lookup: call.Lookup}, nil
}

// start begins the call's timeout.
func (c *grpcCall) start(ctx context.Context) context.CancelFunc {
	var cancel context.CancelFunc
	c.ctx, cancel = context.WithTimeout(metadata.NewOutgoingContext(ctx, c.md), time.Duration(c.timeout*float64(time.Second)))
	return cancel
}

// Send performs the call. A status the server sent, OK or not, is a
// response. One grpc-go made up before the server said anything, such as
// UNAVAILABLE for a refused connection, is an error, as a failed HTTP
// request is. A call the server answered is a response even when ctx is
// cancelled, so cancelling a stream keeps what arrived, with the status
// CANCELLED.
func (g *GRPC) Send(ctx context.Context, call Call) (*model.Response, error) {
	defer call.Stream.end()
	c, err := prepare(call)
	if err != nil {
		return nil, err
	}
	target, err := model.ParseGRPCTarget(c.req.URL)
	if err != nil {
		return nil, err
	}
	method, err := findMethodName(c.payload.Method)
	if err != nil {
		return nil, err
	}
	var (
		files *protoregistry.Files
		md    protoreflect.MethodDescriptor
		in    []proto.Message
	)
	if !c.payload.Protos.Reflection() {
		if files, err = g.protos.load(g.Root, c.payload.Protos); err != nil {
			return nil, err
		}
		// With the schema in hand, a bad method or message is reported
		// before dialing.
		if md, in, err = c.messages(files, method); err != nil {
			return nil, err
		}
	}
	// The timeout starts after the proto files compile.
	cancel := c.start(ctx)
	defer cancel()

	trace := newGRPCTrace(call.OnTrace, target.TLS)
	trace.method = "/" + method.service + "/" + method.name
	started := time.Now()
	conn, err := g.dial(c.ctx, c, target, trace)
	if err != nil {
		trace.end(model.TraceFailed)
		return nil, c.explain(ctx, err)
	}
	defer conn.Close()
	if files == nil {
		if files, err = reflectFiles(c.ctx, conn, []string{method.service}); err != nil {
			trace.end(model.TraceFailed)
			return nil, c.explain(ctx, err)
		}
		if md, in, err = c.messages(files, method); err != nil {
			trace.end(model.TraceFailed)
			return nil, err
		}
	}
	if call.Stream != nil {
		c.ctx = metadata.NewOutgoingContext(ctx, c.md)
		var refusal error
		if !md.IsStreamingClient() {
			refusal = fmt.Errorf("%s is a %s method, so it takes only the message it started with", md.Name(), streamingOf(md))
		}
		call.Stream.start(c.streamParser(md, files), refusal)
	}

	marshal := protojson.MarshalOptions{EmitDefaultValues: true, Resolver: typesOf(files)}
	var live *progress
	if call.OnUpdate != nil {
		live = &progress{report: func(header metadata.MD, messages [][]byte) {
			call.OnUpdate(&model.Response{
				Proto:           "gRPC",
				Headers:         metadataHeaders(header),
				Body:            messagesBody(md, messages),
				BodyContentType: "application/json",
				Elapsed:         time.Since(started),
				URL:             c.req.URL,
			})
		}}
	}
	ex := invoke(c.ctx, conn, md, in, call.Stream, trace, g.maxResponse, marshal, live)
	live.stop()
	if !trace.answered() {
		trace.end(model.TraceFailed)
		return nil, c.explain(ctx, ex.status.Err())
	}
	switch {
	case ctx.Err() != nil && !ex.trailed:
		ex.status = status.New(codes.Canceled, "the call was cancelled")
	case ex.status.Code() == codes.Canceled && c.atDeadline(ex.status.Err()):
		ex.status = status.FromContextError(context.DeadlineExceeded)
	}
	trace.end(model.TraceComplete)
	resp := responseOf(md, ex, marshal)
	resp.Elapsed = time.Since(started)
	resp.ReceivedAt = time.Now()
	resp.Trace = trace.trace()
	resp.URL = c.req.URL
	return resp, nil
}

// streamParser reads the messages sent through an open stream, substituting
// variables as the request's own message was.
func (c *grpcCall) streamParser(md protoreflect.MethodDescriptor, files *protoregistry.Files) func(string) ([]proto.Message, error) {
	return func(text string) ([]proto.Message, error) {
		if c.req.Options.SubstituteBodyVariables {
			text = model.Substitute(text, c.lookup)
		}
		return parseMessages(md, text, files)
	}
}

// Describe lists the methods of the request's schema: its proto files, which
// need no server, or whatever the server's reflection service offers.
func (g *GRPC) Describe(ctx context.Context, call Call) (Schema, error) {
	// Proto paths are never substituted, so proto files are listed without
	// resolving the address and metadata they don't use.
	if p, ok := call.Request.Payload.(model.GRPC); ok && !p.Protos.Reflection() {
		files, err := g.protos.load(g.Root, p.Protos)
		if err != nil {
			return Schema{}, err
		}
		return schemaOf(files), nil
	}
	c, err := prepare(call)
	if err != nil {
		return Schema{}, err
	}
	cancel := c.start(ctx)
	defer cancel()
	target, err := model.ParseGRPCTarget(c.req.URL)
	if err != nil {
		return Schema{}, err
	}
	budget := min(describeDialTimeout, time.Duration(c.timeout*float64(time.Second)))
	dialCtx, cancelDial := context.WithTimeout(c.ctx, budget)
	defer cancelDial()
	conn, err := g.dial(dialCtx, c, target, newGRPCTrace(nil, target.TLS))
	// The budget running out fails whatever step the dial was on, such as
	// the TLS handshake, so the dial's error needn't say it was the budget.
	if err != nil && errors.Is(dialCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		return Schema{}, fmt.Errorf("couldn't connect to %s within %v", target, budget)
	}
	if err != nil {
		return Schema{}, c.explain(ctx, err)
	}
	defer conn.Close()
	files, err := reflectFiles(c.ctx, conn, nil)
	if err != nil {
		return Schema{}, c.explain(ctx, err)
	}
	return schemaOf(files), nil
}

// methodRef is a method name as typed, split into service and method.
type methodRef struct{ service, name string }

// findMethodName reads "pkg.Service/Method", also accepting a leading slash
// and "pkg.Service.Method" as grpcurl does.
func findMethodName(text string) (methodRef, error) {
	text = strings.TrimPrefix(strings.TrimSpace(text), "/")
	if text == "" {
		return methodRef{}, errors.New("choose a method to call")
	}
	service, name, ok := strings.Cut(text, "/")
	if !ok {
		i := strings.LastIndexByte(text, '.')
		if i < 0 {
			return methodRef{}, fmt.Errorf("%q isn't a method: use package.Service/Method", text)
		}
		service, name = text[:i], text[i+1:]
	}
	if service == "" || name == "" || strings.Contains(name, "/") {
		return methodRef{}, fmt.Errorf("%q isn't a method: use package.Service/Method", text)
	}
	return methodRef{service: service, name: name}, nil
}

// lookup finds ref in files. The error lists what there is instead.
func (ref methodRef) lookup(files *protoregistry.Files) (protoreflect.MethodDescriptor, error) {
	d, err := files.FindDescriptorByName(protoreflect.FullName(ref.service))
	service, ok := d.(protoreflect.ServiceDescriptor)
	if err != nil || !ok {
		var names []string
		for _, m := range schemaOf(files).Methods {
			service, _, _ := strings.Cut(m.Name, "/")
			if len(names) == 0 || names[len(names)-1] != service {
				names = append(names, service)
			}
		}
		return nil, fmt.Errorf("no service %s%s", ref.service, listing("services", names))
	}
	md := service.Methods().ByName(protoreflect.Name(ref.name))
	if md == nil {
		var names []string
		methods := service.Methods()
		for i := range methods.Len() {
			names = append(names, string(methods.Get(i).Name()))
		}
		return nil, fmt.Errorf("%s has no method %s%s", ref.service, ref.name, listing("methods", names))
	}
	return md, nil
}

func listing(what string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	const most = 6
	if len(names) > most {
		names = append(names[:most:most], "…")
	}
	return " (" + what + ": " + strings.Join(names, ", ") + ")"
}

// messages finds the call's method in files and decodes its message text.
func (c *grpcCall) messages(files *protoregistry.Files, ref methodRef) (protoreflect.MethodDescriptor, []proto.Message, error) {
	md, err := ref.lookup(files)
	if err != nil {
		return nil, nil, err
	}
	in, err := parseMessages(md, c.payload.Message, files)
	return md, in, err
}

// parseMessages decodes message text for method md: one JSON object, or for
// a client-streaming or bidi method an array of them. Blank is one empty
// message, and an object given to a streaming method is a stream of one.
func parseMessages(md protoreflect.MethodDescriptor, text string, files *protoregistry.Files) ([]proto.Message, error) {
	text = strings.TrimSpace(text)
	streaming := streamingOf(md)
	var raws []json.RawMessage
	switch {
	case text == "":
		raws = []json.RawMessage{json.RawMessage("{}")}
	case !json.Valid([]byte(text)):
		var first json.RawMessage
		if dec := json.NewDecoder(strings.NewReader(text)); dec.Decode(&first) == nil && dec.More() {
			return nil, errors.New("the message holds several JSON values: put a stream's messages in an array")
		}
		var probe any
		return nil, fmt.Errorf("the message isn't valid JSON: %v", json.Unmarshal([]byte(text), &probe))
	case text[0] == '[':
		if !streaming.clientStreams() {
			return nil, fmt.Errorf("%s is a %s method and takes one message, not an array", md.Name(), streaming)
		}
		if err := json.Unmarshal([]byte(text), &raws); err != nil {
			return nil, fmt.Errorf("the message isn't an array of objects: %v", err)
		}
	default:
		raws = []json.RawMessage{json.RawMessage(text)}
	}
	options := protojson.UnmarshalOptions{Resolver: typesOf(files)}
	out := make([]proto.Message, len(raws))
	for i, raw := range raws {
		msg := dynamicpb.NewMessage(md.Input())
		if err := options.Unmarshal(raw, msg); err != nil {
			if len(raws) > 1 {
				return nil, fmt.Errorf("message %d isn't a %s: %v", i+1, md.Input().FullName(), err)
			}
			return nil, fmt.Errorf("the message isn't a %s: %v", md.Input().FullName(), err)
		}
		out[i] = msg
	}
	return out, nil
}

// typesOf resolves message types from files, then from the types compiled
// into Posting (well-known types and the common status details).
func typesOf(files *protoregistry.Files) *resolver {
	return &resolver{files: dynamicpb.NewTypes(files)}
}

type resolver struct{ files *dynamicpb.Types }

func (r *resolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	if t, err := r.files.FindExtensionByName(field); err == nil {
		return t, nil
	}
	return protoregistry.GlobalTypes.FindExtensionByName(field)
}

func (r *resolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	if t, err := r.files.FindExtensionByNumber(message, field); err == nil {
		return t, nil
	}
	return protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
}

func (r *resolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	if t, err := r.files.FindMessageByName(message); err == nil {
		return t, nil
	}
	return protoregistry.GlobalTypes.FindMessageByName(message)
}

func (r *resolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if t, err := r.files.FindMessageByURL(url); err == nil {
		return t, nil
	}
	return protoregistry.GlobalTypes.FindMessageByURL(url)
}

// outgoingMetadata turns enabled headers and auth into the call's metadata.
// Keys are lowercased, -bin values are base64 decoded, and basic and bearer
// auth become authorization metadata, replacing any header of that name.
func outgoingMetadata(headers []model.KeyValue, auth model.Auth) (metadata.MD, error) {
	md := metadata.MD{}
	for _, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h.Name))
		if !h.Enabled || key == "" {
			continue
		}
		if err := checkMetadataKey(key); err != nil {
			return nil, err
		}
		value := h.Value
		if strings.HasSuffix(key, "-bin") {
			decoded, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				if decoded, err = base64.RawStdEncoding.DecodeString(value); err != nil {
					return nil, fmt.Errorf("metadata %s ends in -bin, so its value must be base64: %v", key, err)
				}
			}
			value = string(decoded)
		}
		md.Append(key, value)
	}
	switch auth.Type {
	case model.AuthBasic:
		md.Set("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth.Username+":"+auth.Password)))
	case model.AuthBearer:
		md.Set("authorization", "Bearer "+auth.Token)
	}
	return md, nil
}

// checkMetadataKey rejects keys gRPC reserves for itself or can't carry.
func checkMetadataKey(key string) error {
	switch key {
	case "connection", "keep-alive", "proxy-connection", "transfer-encoding", "upgrade", "host":
		return fmt.Errorf("metadata %s is an HTTP/1 header, which gRPC can't send", key)
	}
	switch {
	case key == "content-type" || key == "te" || strings.HasPrefix(key, "grpc-") || strings.HasPrefix(key, ":"):
		return fmt.Errorf("metadata %s is set by gRPC itself", key)
	case strings.Trim(key, "abcdefghijklmnopqrstuvwxyz0123456789-_.") != "":
		return fmt.Errorf("metadata key %q may only hold letters, digits, -, _ and .", key)
	}
	return nil
}

// dial connects to the call's target and waits until the connection is
// ready, so a server that can't be reached fails here, with the reason,
// rather than as a status on the call. The TLS settings' files are read
// afresh for each TLS connection, so fixing them needs no restart.
func (g *GRPC) dial(ctx context.Context, c *grpcCall, target model.GRPCTarget, trace *grpcTrace) (*grpc.ClientConn, error) {
	failure := &dialFailure{}
	creds := insecure.NewCredentials()
	if target.TLS {
		material, err := loadTLS(g.TLS)
		if err != nil {
			return nil, err
		}
		creds = &tracedTLS{TransportCredentials: credentials.NewTLS(material.config(c.req.Options.VerifySSL)), trace: trace, failure: failure}
	}
	userAgent := g.UserAgent
	if c.userAgent != "" {
		userAgent = c.userAgent
	}
	dialer := &net.Dialer{}
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			trace.begin(model.TraceConnect)
			network := "tcp"
			if target.Socket != "" {
				network, addr = "unix", target.Socket
			}
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				failure.record(err, false)
				return nil, err
			}
			trace.end(model.TraceComplete)
			return conn, nil
		}),
		grpc.WithStatsHandler(trace),
		grpc.WithUserAgent(userAgent),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxBodyBytes)),
	}
	// grpc-go checks a TLS certificate against the authority.
	if authority := strings.TrimSpace(c.payload.Authority); authority != "" {
		options = append(options, grpc.WithAuthority(authority))
	}
	conn, err := grpc.NewClient("passthrough:///"+target.Authority, options...)
	if err != nil {
		return nil, err
	}
	conn.Connect()
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return conn, nil
		case connectivity.TransientFailure, connectivity.Shutdown:
			conn.Close()
			return nil, failure.err(target)
		}
		if !conn.WaitForStateChange(ctx, state) {
			conn.Close()
			if failure.failed() {
				return nil, failure.err(target)
			}
			return nil, ctx.Err()
		}
	}
}

// dialFailure keeps the first reason a connection attempt failed. grpc-go
// retries in the background, so later attempts may fail too.
type dialFailure struct {
	mu        sync.Mutex
	cause     error
	handshake bool
}

func (f *dialFailure) failed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cause != nil
}

func (f *dialFailure) record(err error, handshake bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cause == nil {
		f.cause, f.handshake = err, handshake
	}
}

func (f *dialFailure) err(target model.GRPCTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.cause == nil && (target.TLS || target.Socket != ""):
		return fmt.Errorf("couldn't connect to %s", target)
	case f.cause == nil:
		// The connection opened, then closed before the server spoke
		// HTTP/2: a TLS server hanging up on a plaintext client.
		return fmt.Errorf("%s closed the connection; if it uses TLS, use grpcs://%s", target.Authority, target.Authority)
	case f.handshake:
		var certErr *tls.CertificateVerificationError
		if errors.As(f.cause, &certErr) {
			return fmt.Errorf("SSL certificate verification failed: %v (turn off Verify SSL in Options to skip it)", certErr.Err)
		}
		// A record that isn't TLS at all is a plaintext server answering.
		var recordErr tls.RecordHeaderError
		if errors.As(f.cause, &recordErr) {
			return fmt.Errorf("TLS handshake with %s failed: %v; if the server is plaintext, use grpc://%s", target.Authority, f.cause, target.Authority)
		}
		return fmt.Errorf("TLS handshake with %s failed: %v", target.Authority, f.cause)
	}
	var dnsErr *net.DNSError
	if errors.As(f.cause, &dnsErr) {
		return fmt.Errorf("couldn't resolve host %q", dnsErr.Name)
	}
	var opErr *net.OpError
	if errors.As(f.cause, &opErr) && opErr.Op == "dial" {
		return fmt.Errorf("couldn't connect to %s: %v", target, opErr.Err)
	}
	return fmt.Errorf("couldn't connect to %s: %v", target, f.cause)
}

// tracedTLS times the TLS handshake and keeps its error.
type tracedTLS struct {
	credentials.TransportCredentials
	trace   *grpcTrace
	failure *dialFailure
}

func (t *tracedTLS) ClientHandshake(ctx context.Context, authority string, raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	t.trace.begin(model.TraceTLS)
	conn, info, err := t.TransportCredentials.ClientHandshake(ctx, authority, raw)
	if err != nil {
		t.failure.record(err, true)
		return nil, nil, err
	}
	t.trace.end(model.TraceComplete)
	return conn, info, nil
}

func (t *tracedTLS) Clone() credentials.TransportCredentials {
	return &tracedTLS{TransportCredentials: t.TransportCredentials.Clone(), trace: t.trace, failure: t.failure}
}

// explain turns an error from dialing or calling into one worth showing.
func (c *grpcCall) explain(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(c.ctx.Err(), context.DeadlineExceeded) || c.atDeadline(err) {
		return fmt.Errorf("request timed out after %gs", c.timeout)
	}
	if s, ok := status.FromError(err); ok {
		return fmt.Errorf("%s: %s", model.GRPCCodeName(int(s.Code())), s.Message())
	}
	return err
}

// atDeadline reports whether err ended the call as its deadline arrived.
// The server enforces the deadline Posting sends it, so it may reset the
// call a moment before Posting's own deadline passes.
func (c *grpcCall) atDeadline(err error) bool {
	if code := status.Code(err); code != codes.Canceled && code != codes.DeadlineExceeded {
		return false
	}
	deadline, ok := c.ctx.Deadline()
	return ok && time.Until(deadline) < 100*time.Millisecond
}

// exchange is what came back from one call.
type exchange struct {
	header, trailer metadata.MD
	// messages are the server's messages as JSON.
	messages [][]byte
	status   *status.Status
	// trailed is whether the server sent trailers, and so status.
	trailed bool
}

// invoke runs a call of any shape. It sends every message, then those sent
// through open while it is open, and closes its side on another goroutine
// while it reads until the server ends the call, so a server that replies as
// it reads, filling the flow-control window, never waits on Posting. Each
// message read goes to live as it arrives. Once the messages read pass limit
// bytes, it keeps those before and cancels the call.
func invoke(ctx context.Context, conn *grpc.ClientConn, md protoreflect.MethodDescriptor, in []proto.Message, open *Stream, trace *grpcTrace, limit int, marshal protojson.MarshalOptions, live *progress) exchange {
	desc := &grpc.StreamDesc{StreamName: string(md.Name()), ServerStreams: md.IsStreamingServer(), ClientStreams: md.IsStreamingClient()}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var ex exchange
	stream, err := conn.NewStream(ctx, desc, trace.method)
	if err != nil {
		ex.status = status.Convert(err)
		return ex
	}
	sending := open
	if !md.IsStreamingClient() {
		sending = nil
	}
	sent := make(chan error, 1)
	go func() {
		err := sendAll(ctx, stream, in, sending)
		if err != nil {
			cancel()
		}
		trace.begin(model.TraceReceiveHeaders)
		sent <- err
	}()
	var truncated *status.Status
	received := 0
	for {
		out := dynamicpb.NewMessage(md.Output())
		if err = stream.RecvMsg(out); err != nil {
			break
		}
		if received += proto.Size(out); received > limit {
			truncated = status.Newf(codes.Canceled, "response truncated at %s", model.FormatBytes(limit))
			cancel()
			break
		}
		message := marshalJSON(marshal, out)
		ex.messages = append(ex.messages, message)
		if len(ex.messages) == 1 {
			ex.header, _ = stream.Header()
		}
		live.add(ex.header, message)
	}
	if err == io.EOF {
		err = nil
	}
	open.end()
	sendErr := <-sent
	ex.header, _ = stream.Header()
	ex.trailer = stream.Trailer()
	ex.trailed = trace.trailed()
	switch {
	case truncated != nil:
		ex.status = truncated
	case sendErr != nil && (err == nil || status.Code(err) == codes.Canceled):
		// A failed send cancels the call, so the read may end saying only
		// that.
		ex.status = status.Convert(sendErr)
	default:
		ex.status = status.Convert(err)
	}
	return ex
}

// sendAll sends every message, then those sent through open until it is
// closed, and closes the client's side. io.EOF from SendMsg means the server
// ended the call early, and RecvMsg has its status.
func sendAll(ctx context.Context, stream grpc.ClientStream, in []proto.Message, open *Stream) error {
	for {
		for _, msg := range in {
			if err := stream.SendMsg(msg); err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
		}
		if open == nil {
			break
		}
		if in = open.next(ctx); len(in) == 0 {
			break
		}
	}
	return stream.CloseSend()
}

// responseOf shows an exchange the server answered: its status, metadata
// and messages as JSON. The body follows the method's shape: an object for
// a method that returns one message, an array for one that streams them.
// A failed call with no messages shows its status, with its details.
func responseOf(md protoreflect.MethodDescriptor, ex exchange, marshal protojson.MarshalOptions) *model.Response {
	body := messagesBody(md, ex.messages)
	if ex.status.Code() != codes.OK && len(ex.messages) == 0 {
		body = indentJSON(statusBody(ex.status, marshal))
	}
	var trailers []model.Header
	if ex.trailed {
		trailers = append(trailers, model.Header{Name: "grpc-status", Value: fmt.Sprint(int(ex.status.Code()))})
		if msg := ex.status.Message(); msg != "" {
			trailers = append(trailers, model.Header{Name: "grpc-message", Value: msg})
		}
	}
	return &model.Response{
		Proto:           "gRPC",
		Headers:         metadataHeaders(ex.header),
		Trailers:        append(trailers, metadataHeaders(ex.trailer)...),
		Body:            body,
		BodyContentType: "application/json",
		GRPC:            &model.GRPCStatus{Code: int(ex.status.Code()), Message: ex.status.Message()},
	}
}

// messagesBody is the server's messages, each JSON, as the body shows them:
// an array for a method that streams them, otherwise the one message.
func messagesBody(md protoreflect.MethodDescriptor, messages [][]byte) []byte {
	if !md.IsStreamingServer() {
		if len(messages) == 0 {
			return nil
		}
		return indentJSON(messages[0])
	}
	return indentJSON(append(append([]byte{'['}, bytes.Join(messages, []byte{','})...), ']'))
}

func indentJSON(data []byte) []byte {
	var pretty bytes.Buffer
	if json.Indent(&pretty, data, "", "  ") != nil {
		return data
	}
	return pretty.Bytes()
}

func marshalJSON(options protojson.MarshalOptions, msg proto.Message) []byte {
	data, err := options.Marshal(msg)
	if err != nil {
		return []byte(jsonString(fmt.Sprintf("couldn't show this message: %v", err)))
	}
	return data
}

// statusBody is {"code", "message", "details"}, with each detail decoded
// when its type is known and left as base64 when it isn't.
func statusBody(s *status.Status, marshal protojson.MarshalOptions) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, `{"code":%s,"message":%s`, jsonString(model.GRPCCodeName(int(s.Code()))), jsonString(s.Message()))
	if details := s.Proto().GetDetails(); len(details) > 0 {
		b.WriteString(`,"details":[`)
		for i, detail := range details {
			if i > 0 {
				b.WriteByte(',')
			}
			b.Write(detailJSON(detail, marshal))
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.Bytes()
}

func detailJSON(detail *anypb.Any, marshal protojson.MarshalOptions) []byte {
	if data, err := marshal.Marshal(detail); err == nil {
		return data
	}
	return []byte(fmt.Sprintf(`{"@type":%s,"value":%s}`, jsonString(detail.GetTypeUrl()), jsonString(base64.StdEncoding.EncodeToString(detail.GetValue()))))
}

// metadataHeaders lists metadata sorted by key. Binary values are shown in
// base64, as they are sent.
func metadataHeaders(md metadata.MD) []model.Header {
	keys := make([]string, 0, len(md))
	for key := range md {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []model.Header
	for _, key := range keys {
		for _, value := range md[key] {
			if strings.HasSuffix(key, "-bin") {
				value = base64.StdEncoding.EncodeToString([]byte(value))
			}
			out = append(out, model.Header{Name: key, Value: value})
		}
	}
	return out
}
