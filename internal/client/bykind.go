package client

import (
	"context"
	"fmt"

	"github.com/darrenburns/posting/internal/model"
)

// ByKind sends each request with the sender for its kind.
type ByKind struct {
	HTTP Sender // every kind carried over HTTP
	GRPC Sender
}

func (k ByKind) Send(ctx context.Context, call Call) (*model.Response, error) {
	kind := call.Request.Kind()
	sender := k.For(kind)
	if sender == nil {
		return nil, fmt.Errorf("no sender for %s requests", kind.Label)
	}
	return sender.Send(ctx, call)
}

// For is the sender for kind, or nil when there is none.
func (k ByKind) For(kind *model.Kind) Sender {
	switch {
	case kind.OverHTTP():
		return k.HTTP
	case kind == model.GRPCKind:
		return k.GRPC
	}
	return nil
}
