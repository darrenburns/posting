// Package client defines the boundary between Posting's UI and whatever
// actually performs requests.
//
// The UI only ever talks to a Sender, and to a Describer for the methods a
// gRPC server offers. HTTP sends HTTP-carried requests and GRPC sends gRPC
// calls; ByKind picks between them.
package client

import (
	"context"

	"github.com/darrenburns/posting/internal/model"
)

// Call is everything a Sender needs to perform one request.
type Call struct {
	// Request is the request exactly as edited, with variables unresolved.
	Request model.Request
	// Variables are the values available for substitution, already merged
	// from environment files and session overrides.
	Variables map[string]string
	// OnTrace, when set, receives progress for each stage of the exchange.
	// It may be called from any goroutine; the UI marshals it safely.
	OnTrace func(model.TraceEvent)
}

// Sender performs requests. Implementations must honour ctx cancellation:
// the UI cancels the context when the user cancels or re-sends.
type Sender interface {
	Send(ctx context.Context, call Call) (*model.Response, error)
}

// Describer lists the methods a gRPC schema offers. It is separate from
// Sender because the UI asks it while a request is being edited, not sent.
type Describer interface {
	// Describe reads call's target, metadata and options the way Send
	// would, and returns every method its schema source knows. A request
	// with proto files is described without dialing.
	Describe(ctx context.Context, call Call) (Schema, error)
}

// Schema is a gRPC schema as the UI needs it, without protobuf types.
type Schema struct {
	// Methods are sorted by name. The reflection service isn't listed.
	Methods []Method
}

// Method is one method of a gRPC service.
type Method struct {
	Name      string // "acme.users.v1.UserService/GetUser"
	Streaming Streaming
	Input     string // "acme.users.v1.GetUserRequest"
	Output    string
	// Template is a JSON message for the method with every field at a
	// placeholder value: an object, or for client-streaming and bidi
	// methods an array holding one.
	Template string
}

// Streaming is which sides of a call send a stream of messages.
type Streaming uint8

const (
	Unary Streaming = iota
	ServerStream
	ClientStream
	BidiStream
)

func (s Streaming) String() string {
	return [...]string{"unary", "server stream", "client stream", "bidi stream"}[s]
}

// clientStreams reports whether the client sends a stream of messages.
func (s Streaming) clientStreams() bool { return s == ClientStream || s == BidiStream }

// serverStreams reports whether the server sends a stream of messages.
func (s Streaming) serverStreams() bool { return s == ServerStream || s == BidiStream }

// SenderFunc adapts a function to the Sender interface.
type SenderFunc func(ctx context.Context, call Call) (*model.Response, error)

// Send calls f.
func (f SenderFunc) Send(ctx context.Context, call Call) (*model.Response, error) {
	return f(ctx, call)
}

// Lookup returns a variable lookup function for model.Substitute.
func (c Call) Lookup(name string) (string, bool) {
	value, ok := c.Variables[name]
	return value, ok
}
