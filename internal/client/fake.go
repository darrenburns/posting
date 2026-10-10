package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/darrenburns/posting/v3/internal/model"
)

// Fake is a Sender and Describer that never touches the network. It walks
// through every trace stage with small delays and returns a response
// echoing the request, so the whole UI can be exercised without a server.
type Fake struct {
	// StageDelay is how long each trace stage takes. Zero means no delay.
	StageDelay time.Duration
}

// Send simulates a request.
func (f Fake) Send(ctx context.Context, call Call) (*model.Response, error) {
	if call.Request.Kind() == model.GRPCKind {
		return f.sendGRPC(ctx, call)
	}
	started := time.Now()
	req, ok := model.Lower(call.Request)
	if !ok {
		return nil, fmt.Errorf("%s requests aren't sent over HTTP", call.Request.Kind().Label)
	}
	resolvedURL := model.Substitute(model.ResolvePathParams(req.URL, req.PathParams), call.Lookup)
	parsed, err := url.Parse(resolvedURL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", resolvedURL)
	}
	trace, err := f.walk(ctx, call, parsed.Scheme == "https")
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	for _, h := range req.Headers {
		if h.Enabled {
			headers[h.Name] = model.Substitute(h.Value, call.Lookup)
		}
	}
	query := map[string]string{}
	for key, values := range parsed.Query() {
		query[key] = strings.Join(values, ",")
	}
	echo := map[string]any{
		"method":  string(req.Method),
		"url":     resolvedURL,
		"args":    query,
		"headers": headers,
	}
	switch req.Body.Type {
	case model.BodyRaw:
		var decoded any
		if json.Unmarshal([]byte(req.Body.Raw), &decoded) == nil {
			echo["json"] = decoded
		} else {
			echo["data"] = req.Body.Raw
		}
	case model.BodyForm:
		form := map[string]string{}
		for _, item := range req.Body.Form {
			if item.Enabled {
				form[item.Name] = item.Value
			}
		}
		echo["form"] = form
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(echo)
	body := bytes.TrimSpace(encoded.Bytes())

	status, reason := 200, "OK"
	switch req.Method {
	case model.MethodPost:
		status, reason = 201, "Created"
	case model.MethodDelete:
		status, reason = 204, "No Content"
	}
	if strings.Contains(parsed.Path, "404") || strings.Contains(parsed.Path, "missing") {
		status, reason = 404, "Not Found"
	}
	if req.Method == model.MethodHead || status == 204 {
		body = nil
	}

	return &model.Response{
		StatusCode: status,
		Reason:     reason,
		Proto:      "HTTP/1.1",
		Headers: []model.Header{
			{Name: "Content-Type", Value: "application/json; charset=utf-8"},
			{Name: "Content-Length", Value: fmt.Sprint(len(body))},
			{Name: "Date", Value: time.Now().UTC().Format(time.RFC1123)},
			{Name: "Server", Value: "posting-fake/3.0"},
			{Name: "Set-Cookie", Value: "session=fake-session-id; Path=/; HttpOnly"},
		},
		Cookies: []model.Cookie{
			{Name: "session", Value: "fake-session-id", Path: "/", HTTPOnly: true},
		},
		Body:       body,
		Elapsed:    time.Since(started),
		ReceivedAt: time.Now(),
		Trace:      trace,
		URL:        resolvedURL,
		Method:     req.Method,
	}, nil
}

// walk reports every trace stage in turn, skipping TLS without it.
func (f Fake) walk(ctx context.Context, call Call, tls bool) ([]model.TraceEvent, error) {
	var trace []model.TraceEvent
	for _, stage := range model.TraceStages {
		if stage == model.TraceTLS && !tls {
			trace = append(trace, model.TraceEvent{Stage: stage, State: model.TraceSkipped})
			f.report(call, trace[len(trace)-1])
			continue
		}
		f.report(call, model.TraceEvent{Stage: stage, State: model.TraceStarted})
		stageStart := time.Now()
		select {
		case <-ctx.Done():
			f.report(call, model.TraceEvent{Stage: stage, State: model.TraceFailed})
			return nil, ctx.Err()
		case <-time.After(f.StageDelay):
		}
		event := model.TraceEvent{Stage: stage, State: model.TraceComplete, Duration: time.Since(stageStart)}
		trace = append(trace, event)
		f.report(call, event)
	}
	return trace, nil
}

// sendGRPC answers a gRPC call with its own message, or NOT_FOUND for a
// method whose name has "Missing" in it.
func (f Fake) sendGRPC(ctx context.Context, call Call) (*model.Response, error) {
	started := time.Now()
	req, err := model.Resolve(call.Request, call.Lookup)
	if err != nil {
		return nil, err
	}
	target, err := model.ParseGRPCTarget(req.URL)
	if err != nil {
		return nil, err
	}
	payload := req.Payload.(model.GRPC)
	if _, err := findMethodName(payload.Method); err != nil {
		return nil, err
	}
	trace, err := f.walk(ctx, call, target.TLS)
	if err != nil {
		return nil, err
	}
	status := &model.GRPCStatus{}
	body := []byte(strings.TrimSpace(payload.Message))
	if len(body) == 0 {
		body = []byte("{}")
	}
	if strings.Contains(payload.Method, "Missing") {
		status = &model.GRPCStatus{Code: 5, Message: "no such thing"}
		body = []byte(`{"code": "NOT_FOUND", "message": "no such thing"}`)
	}
	if call.Stream != nil {
		body, status = f.echoStream(ctx, call, body)
	}
	return &model.Response{
		Proto:           "gRPC",
		Headers:         []model.Header{{Name: "content-type", Value: "application/grpc"}, {Name: "server", Value: "posting-fake/3.0"}},
		Trailers:        []model.Header{{Name: "grpc-status", Value: fmt.Sprint(status.Code)}},
		Body:            body,
		BodyContentType: "application/json",
		GRPC:            status,
		Elapsed:         time.Since(started),
		ReceivedAt:      time.Now(),
		Trace:           trace,
		URL:             req.URL,
	}, nil
}

// echoStream answers an open stream: it echoes the call's first message and
// each one sent through the stream, as they arrive, until the stream ends.
func (f Fake) echoStream(ctx context.Context, call Call, first []byte) ([]byte, *model.GRPCStatus) {
	var messages [][]byte
	echo := func(message []byte) {
		messages = append(messages, message)
		if call.OnUpdate != nil {
			call.OnUpdate(&model.Response{Proto: "gRPC", Body: messagesJSON(messages), BodyContentType: "application/json"})
		}
	}
	echo(first)
	call.Stream.start(func(text string) ([]proto.Message, error) {
		value := &structpb.Value{}
		if err := protojson.Unmarshal([]byte(text), value); err != nil {
			return nil, fmt.Errorf("the message isn't valid JSON: %v", err)
		}
		return []proto.Message{value}, nil
	}, nil)
	for {
		sent := call.Stream.next(ctx)
		if len(sent) == 0 {
			break
		}
		for _, message := range sent {
			data, _ := protojson.Marshal(message)
			echo(data)
		}
	}
	status := &model.GRPCStatus{}
	if ctx.Err() != nil {
		status = &model.GRPCStatus{Code: 1, Message: "the call was cancelled"}
	}
	return messagesJSON(messages), status
}

func messagesJSON(messages [][]byte) []byte {
	return []byte("[" + string(bytes.Join(messages, []byte(","))) + "]")
}

// Describe returns the same schema whatever is asked: a greeter service
// with one method of each shape.
func (f Fake) Describe(ctx context.Context, call Call) (Schema, error) {
	if err := ctx.Err(); err != nil {
		return Schema{}, err
	}
	const service = "posting.example.v1.Greeter/"
	hello := "{\n  \"name\": \"\"\n}"
	return Schema{Methods: []Method{
		{Name: service + "Chat", Streaming: BidiStream, Input: "posting.example.v1.ChatMessage", Output: "posting.example.v1.ChatMessage", Template: "[\n  {\n    \"text\": \"\"\n  }\n]"},
		{Name: service + "CollectNames", Streaming: ClientStream, Input: "posting.example.v1.HelloRequest", Output: "posting.example.v1.NameCount", Template: "[\n" + indent(hello) + "\n]"},
		{Name: service + "SayHello", Streaming: Unary, Input: "posting.example.v1.HelloRequest", Output: "posting.example.v1.HelloReply", Template: hello},
		{Name: service + "StreamGreetings", Streaming: ServerStream, Input: "posting.example.v1.HelloRequest", Output: "posting.example.v1.HelloReply", Template: hello},
	}}, nil
}

func (f Fake) report(call Call, event model.TraceEvent) {
	if call.OnTrace != nil {
		call.OnTrace(event)
	}
}
