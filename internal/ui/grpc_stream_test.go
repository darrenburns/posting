package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

// streamApp is the test app with a bidi gRPC request open, sending through
// the fake client, whose updates wait in the returned channel.
func streamApp(tt *testing.T) (*App, *Session, chan func()) {
	tt.Helper()
	app := testApp()
	app.sender = client.Fake{}
	req := grpcRequest()
	req.Payload = model.GRPC{Method: "posting.example.v1.Greeter/Chat", Message: `{"text": "hi"}`}
	updates := make(chan func(), 64)
	app.current().dispatch = func(fn func()) { updates <- fn }
	app.openRequest(req)
	return app, app.current(), updates
}

// drainUntil runs dispatched updates until cond holds.
func drainUntil(tt *testing.T, updates chan func(), cond func() bool) {
	tt.Helper()
	timeout := time.After(2 * time.Second)
	for !cond() {
		select {
		case update := <-updates:
			update()
		case <-timeout:
			tt.Fatal("timed out waiting for the exchange")
		}
	}
}

func bodyHas(s *Session, text string) bool {
	return s.response.Peek() != nil && strings.Contains(string(s.response.Peek().Body), text)
}

func TestGRPCOpenStreamTakesMessagesUntilEnded(tt *testing.T) {
	app, s, updates := streamApp(tt)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	if s.phase.Peek() != exchangeSending {
		tt.Fatalf("phase = %v while the stream is open", s.phase.Peek())
	}

	s.payloads[model.KindGRPC].(*grpcEditor).message.SetText(`{"text": "again"}`)
	app.send()
	drainUntil(tt, updates, func() bool { return bodyHas(s, "again") })
	if s.phase.Peek() != exchangeSending {
		tt.Fatal("sending into the open stream ended its call")
	}

	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
	if s.streaming.Peek() != streamNone {
		tt.Errorf("streaming = %v after the call ended", s.streaming.Peek())
	}
	if !bodyHas(s, "hi") || !bodyHas(s, "again") || s.responseStatus.Code != "OK" {
		tt.Errorf("response = %s, status %+v", s.response.Peek().Body, s.responseStatus)
	}
	if got := len(app.history.Peek()); got != 1 {
		tt.Errorf("history has %d entries, want the finished call", got)
	}
}

func TestGRPCStoppingAStreamKeepsWhatArrived(tt *testing.T) {
	app, s, updates := streamApp(tt)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return bodyHas(s, "hi") })
	app.cancelSend()
	drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
	if !bodyHas(s, "hi") || s.responseStatus.Code != "CANCELLED" {
		tt.Errorf("response = %s, status %+v", s.response.Peek().Body, s.responseStatus)
	}
}

func TestGRPCLiveResponseShowsOnScreen(tt *testing.T) {
	app, s, updates := streamApp(tt)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	sc := newScreen(app, snapW, snapH)
	sc.render()
	for _, want := range []string{"STREAM OPEN", `"text": "hi"`} {
		if !sc.shows(want) {
			tt.Errorf("screen doesn't show %q:\n%s", want, sc.renderer.ScreenText())
		}
	}
}

func TestGRPCAuthorityIsEditedOnTheOptionsTab(tt *testing.T) {
	app := testApp()
	req := grpcRequest()
	req.Payload = model.GRPC{Method: "a.B/C", Authority: "books.internal"}
	app.openRequest(req)
	s := app.current()
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	if got := e.authority.GetText(); got != "books.internal" {
		tt.Fatalf("authority field = %q", got)
	}
	e.authority.SetText(" ${HOST} ")
	if got := s.Snapshot().Payload.(model.GRPC).Authority; got != "${HOST}" {
		tt.Errorf("snapshot authority = %q", got)
	}
}
