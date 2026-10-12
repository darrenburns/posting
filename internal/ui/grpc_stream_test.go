package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"
	uv "github.com/darrenburns/ultraviolet"

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
	tt.Cleanup(s.Stop)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	sc := newScreen(app, snapW, snapH)
	sc.render()
	for _, want := range []string{"STREAM OPEN", "Messages", "↑ {", "↓ {", "↓ Received", `"text": "hi"`} {
		if !sc.shows(want) {
			tt.Errorf("screen doesn't show %q:\n%s", want, sc.renderer.ScreenText())
		}
	}
}

func TestGRPCFollowingKeepsLatestMessageVisible(tt *testing.T) {
	app, s, updates := streamApp(tt)
	tt.Cleanup(s.Stop)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	sc := newScreen(app, snapW, snapH)
	sc.focusID(tt, messagesID)
	logText := func() string {
		box := sc.renderer.WidgetByID(messagesID).Visible
		lines := strings.Split(sc.renderer.ScreenText(), "\n")
		var text strings.Builder
		for _, line := range lines[box.Y : box.Y+box.Height] {
			cells := []rune(line)
			text.WriteString(string(cells[box.X:min(box.X+box.Width, len(cells))]))
		}
		return text.String()
	}
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	for i := range 20 {
		e.message.SetText(fmt.Sprintf(`{"n":"%02d"}`, i))
		app.send()
	}
	drainUntil(tt, updates, func() bool { return bodyHas(s, `"19"`) })
	sc.render()
	if !strings.Contains(logText(), `"19"`) {
		tt.Fatalf("following left the newest message below the visible log:\n%s", sc.renderer.ScreenText())
	}

	// Moving within the visible rows must pause following, even at the bottom.
	sc.pressKey(tt, "up")
	sc.pressKey(tt, "up")
	before, body := logText(), s.responseBody.GetText()
	e.message.SetText(`{"n":"20"}`)
	app.send()
	drainUntil(tt, updates, func() bool { return bodyHas(s, `"20"`) })
	sc.render()
	if logText() != before || s.responseBody.GetText() != body || !sc.shows("2 new") {
		tt.Fatalf("arrivals moved the paused log or inspector:\n%s", sc.renderer.ScreenText())
	}
	sc.pressKey(tt, "f")
	if !strings.Contains(logText(), `"20"`) || !sc.shows("Following latest") {
		tt.Fatalf("resuming follow didn't reveal the newest message:\n%s", sc.renderer.ScreenText())
	}
}

func TestGRPCInspectingAMessageSurvivesArrivalsAndCompletion(tt *testing.T) {
	app, s, updates := streamApp(tt)
	tt.Cleanup(s.Stop)
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	e.message.SetText(`{"text":"hi","lines":[` + strings.Repeat("1,", 40) + "2]}")
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	sc := newScreen(app, snapW, snapH)
	sc.focusID(tt, messagesID)
	sc.pressKey(tt, "up") // Inspect the sent message, not the latest received one.
	sc.pressKey(tt, "enter")
	sc.focusID(tt, "resp-body") // The headless renderer doesn't consume RequestFocus.
	sc.pressKey(tt, "G")
	sc.pressKey(tt, "v")
	sc.pressKey(tt, "up")
	text, selected := s.bodyCopyText()
	if !selected || s.responseBodyScroll.Offset.Peek() == 0 {
		tt.Fatal("the test must inspect a selected, scrolled payload")
	}
	cursor, scroll := s.responseBody.CursorIndex.Peek(), s.responseBodyScroll.Offset.Peek()
	e.message.SetText(`{"text":"again"}`)
	app.send()
	drainUntil(tt, updates, func() bool { return bodyHas(s, "again") })
	sc.render()
	check := func() {
		tt.Helper()
		if got, _ := s.bodyCopyText(); got != text || s.responseBody.CursorIndex.Peek() != cursor || s.responseBodyScroll.Offset.Peek() != scroll {
			tt.Fatalf("arriving data changed selection %q → %q, cursor %d → %d, scroll %d → %d", text, got, cursor, s.responseBody.CursorIndex.Peek(), scroll, s.responseBodyScroll.Offset.Peek())
		}
	}
	check()
	if !sc.shows("2 new") {
		tt.Fatalf("the paused log should count arrivals:\n%s", sc.renderer.ScreenText())
	}
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
	sc.render()
	check()
	sc.pressKey(tt, "escape") // Leave visual mode before returning to the log.
	sc.pressKey(tt, "escape")
	sc.focusID(tt, messagesID)
	sc.pressKey(tt, "f")
	if !sc.shows("Following latest") || !strings.Contains(s.responseBody.GetText(), "Call ended: OK") {
		tt.Fatalf("following should select the final event:\n%s", sc.renderer.ScreenText())
	}
	entry := app.history.Peek()[0]
	s.showResponse(entry.Response, &entry)
	sc.render()
	if !sc.shows("↑ {") || !sc.shows("↓ {") || !sc.shows("Call ended: OK") {
		tt.Fatalf("history lost the exchange log:\n%s", sc.renderer.ScreenText())
	}
}

func TestGRPCNarrowLogOpensAndClosesDetailsWithoutCancelling(tt *testing.T) {
	app, s, updates := streamApp(tt)
	tt.Cleanup(s.Stop)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return bodyHas(s, "hi") })
	sc := newScreen(app, 82, snapH)
	sc.focusID(tt, messagesID)
	if sc.visible("resp-body") {
		tt.Fatal("a narrow pane should show the log without a squeezed inspector")
	}
	sc.pressKey(tt, "enter")
	if sc.visible(messagesID) || !sc.visible("resp-body") || !sc.shows(`"text": "hi"`) {
		tt.Fatalf("enter should open the selected payload at full width:\n%s", sc.renderer.ScreenText())
	}
	sc.focusID(tt, "resp-body")
	sc.pressKey(tt, "escape")
	if !sc.visible(messagesID) || s.phase.Peek() != exchangeSending {
		tt.Fatal("escape should return to the log, not cancel the stream")
	}
}

func TestGRPCMessageDividerDragsWithoutChangingTheMessage(tt *testing.T) {
	app := messageSnapshotApp(tt)
	sc := newScreen(app, snapW, snapH)
	sc.focusID(tt, messagesID)
	sc.pressKey(tt, "up")
	message := app.current().responseBody.GetText()
	for _, dx := range []int{-9, 17} {
		list := sc.renderer.WidgetByID(messagesID).Bounds
		inspector := sc.renderer.WidgetByID("resp-body").Bounds
		x, y := list.X+list.Width, list.Y
		divider := sc.renderer.WidgetAt(x, y)
		if divider == nil || divider.ID != "resp-message-split" {
			tt.Fatalf("the bar beside the log isn't a draggable divider: %+v", divider)
		}
		event := t.MouseEvent{X: x, Y: y, LocalX: x - divider.Bounds.X, LocalY: y - divider.Bounds.Y, Button: uv.MouseLeft, ClickCount: 1}
		divider.EventWidget.(t.MouseDownHandler).OnMouseDown(event)
		event.X, event.LocalX = event.X+dx, event.LocalX+dx
		divider.EventWidget.(t.MouseMoveHandler).OnMouseMove(event)
		divider.EventWidget.(t.MouseUpHandler).OnMouseUp(event)
		sc.render()
		listWidth := sc.renderer.WidgetByID(messagesID).Bounds.Width
		inspectorWidth := sc.renderer.WidgetByID("resp-body").Bounds.Width
		if dx < 0 && (listWidth >= list.Width || inspectorWidth <= inspector.Width) ||
			dx > 0 && (listWidth <= list.Width || inspectorWidth >= inspector.Width) {
			tt.Fatalf("drag %d: log width %d → %d, inspector width %d → %d", dx, list.Width, listWidth, inspector.Width, inspectorWidth)
		}
		if app.current().responseBody.GetText() != message {
			tt.Fatal("dragging the divider changed the inspected message")
		}
		tt.Logf("drag %d: log width %d → %d, inspector width %d → %d; message unchanged", dx, list.Width, listWidth, inspector.Width, inspectorWidth)
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

func TestSendingAfterLeavingGRPCSendsTheNewRequest(tt *testing.T) {
	app, s, updates := streamApp(tt)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen })
	app.setKind(model.KindHTTP)
	s.url.SetText("http://example.com/books")
	app.send()
	drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
	if got := s.sent.Kind().ID; got != model.KindHTTP {
		tt.Errorf("the send went to the %s stream, not the HTTP request", got)
	}
}

func messageSnapshotApp(tt *testing.T) *App {
	app := grpcSnapshotApp(tt)
	s := app.current()
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	e.method.SetText("posting.example.v1.Greeter/Chat")
	e.message.SetText("{\n  \"text\": \"And something shorter?\"\n}")
	s.title.Set("Chat about books")
	s.phase.Set(exchangeSending)
	s.streaming.Set(streamOpen)
	s.sent = s.Snapshot()
	at := time.Date(2026, 10, 11, 14, 32, 0, 0, time.Local)
	events := []model.MessageEvent{
		{Kind: model.MessageInfo, Title: "Call started"},
		{Kind: model.MessageSent, Body: []byte(`{"text":"Recommend a science fiction book."}`)},
		{Kind: model.MessageReceived, Body: []byte(`{"text":"Try Dune by Frank Herbert."}`)},
		{Kind: model.MessageReceived, Body: []byte(`{"text":"It follows Paul Atreides on the desert planet Arrakis."}`)},
		{Kind: model.MessageSent, Body: []byte(`{"text":"And something shorter?"}`)},
		{Kind: model.MessageReceived, Body: []byte(`{"text":"Try The Lathe of Heaven by Ursula K. Le Guin."}`)},
	}
	for i := range events {
		events[i].At = at.Add(time.Duration(i) * 350 * time.Millisecond)
		if len(events[i].Body) > 0 {
			events[i].ContentType = "application/json"
		}
	}
	s.showLive(s.sent, &model.Response{Proto: "gRPC", Events: events, Headers: []model.Header{{Name: "content-type", Value: "application/grpc"}}})
	return app
}

func TestSnapshotGRPCMessages(tt *testing.T) {
	app := messageSnapshotApp(tt)
	t.AssertSnapshotNamed(tt, "GRPC_messages_wide", app, snapW, snapH, "A live bidi call with sent and received messages and the latest payload in the side inspector")
	t.AssertSnapshotNamed(tt, "GRPC_messages_narrow", app, 82, snapH, "A narrow response pane uses its full width for the message log")
	sc := newScreen(app, 82, snapH)
	sc.focusID(tt, messagesID)
	sc.pressKey(tt, "enter")
	t.AssertSnapshotNamed(tt, "GRPC_messages_detail", app, 82, snapH, "Enter opens a full-width message inspector; Escape returns to the log")
}
