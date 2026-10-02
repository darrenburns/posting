package client

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/stats"

	"github.com/darrenburns/posting/internal/model"
)

// grpcTrace lays a gRPC call onto the trace stages HTTP uses: connect and
// TLS from the dialer, then metadata out, messages out, metadata in,
// messages in and trailers from the call's stats. Stages only move forward,
// so events from a redial or from the reflection call before the traced one
// can't rewind it. It also records whether the server answered at all.
type grpcTrace struct {
	report func(model.TraceEvent)
	tls    bool
	// method is the traced call's full name, "/pkg.Service/Method".
	method string

	mu      sync.Mutex
	current int // index in model.TraceStages of the stage under way; -1 before connect
	running bool
	began   time.Time
	events  []model.TraceEvent
	heard   bool
}

func newGRPCTrace(report func(model.TraceEvent), tls bool) *grpcTrace {
	return &grpcTrace{report: report, tls: tls, current: -1}
}

// begin finishes the stage under way and starts stage. Stages it passes
// over took no time, apart from TLS on a plaintext connection, which is
// skipped.
//
// Events are reported with the lock held: they come from grpc-go's
// goroutines as well as the caller's, and must arrive in order.
func (g *grpcTrace) begin(stage model.TraceStage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	target := int(stage)
	if target <= g.current {
		return
	}
	if g.running {
		g.emit(g.finishLocked(model.TraceComplete))
	}
	for i := g.current + 1; i < target; i++ {
		event := model.TraceEvent{Stage: model.TraceStages[i], State: model.TraceComplete}
		if model.TraceStages[i] == model.TraceTLS && !g.tls {
			event.State = model.TraceSkipped
		}
		g.events = append(g.events, event)
		g.emit(event)
	}
	g.current, g.running, g.began = target, true, time.Now()
	g.emit(model.TraceEvent{Stage: stage, State: model.TraceStarted})
}

// end finishes the stage under way, as complete or failed.
func (g *grpcTrace) end(state model.TraceState) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running {
		g.emit(g.finishLocked(state))
	}
}

func (g *grpcTrace) finishLocked(state model.TraceState) model.TraceEvent {
	event := model.TraceEvent{Stage: model.TraceStages[g.current], State: state}
	if state == model.TraceComplete {
		event.Duration = time.Since(g.began)
	}
	g.running = false
	if state != model.TraceFailed {
		g.events = append(g.events, event)
	}
	return event
}

func (g *grpcTrace) emit(event model.TraceEvent) {
	if g.report != nil {
		g.report(event)
	}
}

// trace is the finished stages, in order.
func (g *grpcTrace) trace() []model.TraceEvent {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]model.TraceEvent(nil), g.events...)
}

// answered reports whether the server sent the traced call any metadata,
// so a status it ended with is the server's and not one grpc-go made up.
func (g *grpcTrace) answered() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.heard
}

type tracedCall struct{}

func (g *grpcTrace) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	if info.FullMethodName != g.method {
		return ctx
	}
	return context.WithValue(ctx, tracedCall{}, true)
}

func (g *grpcTrace) HandleRPC(ctx context.Context, s stats.RPCStats) {
	if traced, _ := ctx.Value(tracedCall{}).(bool); !traced {
		return
	}
	switch s.(type) {
	case *stats.Begin:
		g.begin(model.TraceSendHeaders)
	case *stats.OutHeader:
		g.begin(model.TraceSendBody)
	case *stats.InHeader:
		g.hear()
		g.begin(model.TraceReceiveBody)
	case *stats.InTrailer:
		g.hear()
		g.begin(model.TraceClosed)
	case *stats.End:
		g.end(model.TraceComplete)
	}
}

func (g *grpcTrace) hear() {
	g.mu.Lock()
	g.heard = true
	g.mu.Unlock()
}

func (g *grpcTrace) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

func (g *grpcTrace) HandleConn(context.Context, stats.ConnStats) {}
