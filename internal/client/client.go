// Package client defines the boundary between Posting's UI and whatever
// actually performs HTTP requests.
//
// The UI only ever talks to a Sender. To wire in a real HTTP client, implement
// Sender (resolve variables, apply auth/options, perform the request, report
// trace progress) and pass it to ui.New in cmd/posting.
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
