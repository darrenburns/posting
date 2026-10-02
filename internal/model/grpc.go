package model

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// GRPC is a call to one gRPC method. It isn't carried over HTTP the way
// GraphQL is: Lower refuses it, and client.GRPC sends it.
type GRPC struct {
	// Method is "package.Service/Method", the call's HTTP/2 path without its
	// leading slash. It is kept as typed, so a half-typed name still saves.
	Method string
	// Message is protojson text: one object for unary and server-streaming
	// methods, an array of objects for client-streaming and bidi methods.
	// Blank means one empty message.
	Message string
	// Protos are where the schema comes from. Empty means server reflection.
	Protos ProtoSet
}

// ProtoSet lists collection-relative schema files. A file ending in
// .protoset, .binpb or .pb is a serialized FileDescriptorSet; any other file
// is .proto source, found relative to ImportPaths (the collection root when
// there are none).
type ProtoSet struct {
	Files       []string `json:",omitempty"`
	ImportPaths []string `json:",omitempty"`
}

// Reflection reports whether the schema is asked of the server.
func (p ProtoSet) Reflection() bool { return len(p.Files) == 0 }

func (g GRPC) Kind() *Kind { return GRPCKind }

func (g GRPC) clone() Payload {
	g.Protos.Files = cloneStrings(g.Protos.Files)
	g.Protos.ImportPaths = cloneStrings(g.Protos.ImportPaths)
	return g
}

func (g GRPC) size() int {
	n := len(g.Method) + len(g.Message)
	for _, path := range append(g.Protos.Files, g.Protos.ImportPaths...) {
		n += len(path)
	}
	return n
}

// resolve substitutes the message like a raw body. The method and proto
// paths are names, not text, so they are never substituted.
func (g GRPC) resolve(_, all func(string) string, opts Options) Payload {
	g = g.clone().(GRPC)
	if opts.SubstituteBodyVariables {
		g.Message = all(g.Message)
	}
	return g
}

// status names the gRPC status code: {Code: "NOT_FOUND", Text: "no book
// 42", Class: StatusClassError}. Only OK is a success.
func (g GRPC) status(resp *Response) Status {
	if resp.GRPC == nil {
		return Status{Code: GRPCCodeName(grpcUnknown), Text: "no gRPC status", Class: StatusClassError}
	}
	s := Status{Code: GRPCCodeName(resp.GRPC.Code), Text: resp.GRPC.Message, Class: StatusClassError}
	if resp.GRPC.Code == 0 {
		s.Class = StatusClassSuccess
	}
	return s
}

// label is "Service/Method": the method without its package.
func (g GRPC) label() string {
	method := strings.TrimPrefix(strings.TrimSpace(g.Method), "/")
	service, name, ok := strings.Cut(method, "/")
	if !ok {
		return method
	}
	if i := strings.LastIndexByte(service, '.'); i >= 0 {
		service = service[i+1:]
	}
	return service + "/" + name
}

// GRPCStatus is the status a gRPC server ended a call with.
type GRPCStatus struct {
	Code    int
	Message string
}

const grpcUnknown = 2

// grpcCodeNames are the canonical names of codes 0 to 16, kept here so model
// doesn't import grpc.
var grpcCodeNames = [...]string{"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT",
	"DEADLINE_EXCEEDED", "NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED",
	"RESOURCE_EXHAUSTED", "FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE",
	"UNIMPLEMENTED", "INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED"}

// GRPCCodeName names gRPC status code c: "NOT_FOUND", or "CODE(c)" for a
// code outside the standard set.
func GRPCCodeName(c int) string {
	if c >= 0 && c < len(grpcCodeNames) {
		return grpcCodeNames[c]
	}
	return fmt.Sprintf("CODE(%d)", c)
}

// GRPCTarget is where a gRPC call goes.
type GRPCTarget struct {
	// Authority is "host:port".
	Authority string
	TLS       bool
}

// ParseGRPCTarget reads a resolved gRPC server address. A scheme decides the
// transport: grpc:// and http:// are plaintext, grpcs:// and https:// are
// TLS. A bare host:port is TLS unless the host is a loopback address, so
// "localhost:50051" works as typed and a remote server never gets
// credentials in plaintext by accident. Without a port, TLS uses 443 and
// plaintext 80.
func ParseGRPCTarget(address string) (GRPCTarget, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return GRPCTarget{}, errors.New("enter the server's address, e.g. localhost:50051")
	}
	rest, tls, explicit := address, false, false
	if scheme, after, ok := strings.Cut(address, "://"); ok {
		switch strings.ToLower(scheme) {
		case "grpc", "http":
		case "grpcs", "https":
			tls = true
		default:
			return GRPCTarget{}, fmt.Errorf("unsupported scheme %q: use grpc:// for plaintext or grpcs:// for TLS", scheme)
		}
		rest, explicit = after, true
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		if strings.Trim(rest[i:], "/") != "" {
			return GRPCTarget{}, fmt.Errorf("%q has a path: the address is just host:port, and the method goes in the Method field", address)
		}
		rest = rest[:i]
	}
	host, port, err := net.SplitHostPort(rest)
	if err != nil {
		host, port = strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]"), ""
	}
	if host == "" {
		return GRPCTarget{}, fmt.Errorf("%q has no host", address)
	}
	if !explicit {
		tls = !isLoopback(host)
	}
	if port == "" {
		port = "80"
		if tls {
			port = "443"
		}
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return GRPCTarget{}, fmt.Errorf("%q has an invalid port %q", address, port)
	}
	return GRPCTarget{Authority: net.JoinHostPort(host, port), TLS: tls}, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string(nil), in...)
}

func exampleGRPC() Request {
	r := NewRequest()
	r.Payload = GRPC{
		Method:  "acme.users.v1.UserService/GetUser",
		Message: "{\"id\": \"${USER_ID}\"}\n",
		Protos:  ProtoSet{Files: []string{"protos/acme/users/v1/users.proto"}, ImportPaths: []string{"protos"}},
	}
	r.Name = "Get user over gRPC"
	r.Description = "Fetch a user through the gRPC API."
	r.URL = "localhost:50051"
	r.Headers = []KeyValue{{Name: "x-tenant", Value: "core", Enabled: true}}
	r.Auth = Auth{Type: AuthBearer, Token: "${API_TOKEN}"}
	r.Options.VerifySSL = false
	r.Options.TimeoutSeconds = 10
	r.File = "users/get-user-grpc.posting.yaml"
	return Normalize(r)
}
