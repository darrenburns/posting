package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestSSEInspectFollowStopAndHistory(tt *testing.T) {
	for _, tc := range []struct {
		theme string
		width int
	}{{"galaxy", 130}, {"catppuccin-latte", 82}} {
		tt.Run(tc.theme, func(tt *testing.T) {
			frames := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				for {
					select {
					case frame := <-frames:
						io.WriteString(w, frame)
						w.(http.Flusher).Flush()
					case <-r.Context().Done():
						return
					}
				}
			}))
			defer server.Close()
			app := homeApp(tc.theme)
			app.sender = client.NewHTTP("posting-test", client.TLSSettings{})
			req := model.NewRequest()
			req.Name, req.URL = "Deployment events", server.URL+"/events"
			app.openRequest(req)
			s := app.current()
			defer s.Stop()
			updates := make(chan func(), 64)
			s.dispatch = func(fn func()) { updates <- fn }
			p := t.NewPilot(tt, app, tc.width, snapH)
			p.Press("ctrl+j")
			drainUntil(tt, updates, func() bool { return s.live })
			p.Advance(0)
			stopHint := "stops the stream"
			if tc.width == 82 {
				stopHint = "esc Stop"
			}
			for _, want := range []string{"Streaming", "Events", "Stream connected", stopHint} {
				if !strings.Contains(p.ScreenText(), want) {
					tt.Fatalf("connected screen lacks %q:\n%s", want, p.ScreenText())
				}
			}
			if strings.Contains(p.ScreenText(), "sends the message") || strings.Contains(p.ScreenText(), "Finish sending") {
				tt.Fatal("SSE offered gRPC sending controls")
			}
			frames <- "data:\n\n"
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 2 })
			p.Advance(0)
			t.RequestFocus(messagesID)
			p.Advance(0)
			p.Press("enter")
			if !strings.Contains(p.ScreenText(), "Empty event data") {
				tt.Fatalf("empty event has no inspector:\n%s", p.ScreenText())
			}
			p.Press("escape")
			if s.phase.Peek() != exchangeSending || strings.Contains(p.ScreenText(), "esc Back to log") {
				tt.Fatalf("leaving an empty event inspector didn't return to the live log:\n%s", p.ScreenText())
			}
			p.Press("f")
			frames <- "event: deployment\nid: deploy-42\ndata: {\"service\":\"api\",\"progress\":25}\n\n"
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 3 })
			p.Advance(0)
			t.RequestFocus(messagesID)
			p.Advance(0)
			p.Press("enter")
			for _, want := range []string{"deployment", "ID: deploy-42", `"progress": 25`} {
				if !strings.Contains(p.ScreenText(), want) {
					tt.Fatalf("inspector lacks %q:\n%s", want, p.ScreenText())
				}
			}
			p.Press("t") // The event payload uses the existing JSON tree.
			if _, ok := p.Bounds(responseJSONQueryID); !ok {
				tt.Fatal("SSE JSON data has no tree inspector")
			}
			frames <- "event: deployment\nid: deploy-43\ndata: {\"service\":\"api\",\"progress\":80}\n\n"
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 4 })
			p.Advance(0)
			if !strings.Contains(p.ScreenText(), "1 new") || !strings.Contains(p.TextOf("resp-body"), "25") || strings.Contains(p.TextOf("resp-body"), "80") {
				tt.Fatalf("an arrival interrupted inspection:\n%s", p.ScreenText())
			}
			p.Press("escape") // Back to log, without stopping the stream.
			if s.phase.Peek() != exchangeSending {
				tt.Fatal("leaving the inspector stopped SSE")
			}
			p.Press("f")
			if !strings.Contains(p.ScreenText(), "Following latest") {
				tt.Fatal("could not resume following")
			}
			p.Press("escape") // Stop from the log, keeping the exchange.
			drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
			p.Advance(0)
			if strings.Contains(p.ScreenText(), "Streaming") || !strings.Contains(p.ScreenText(), "Stream stopped") {
				tt.Fatalf("stopped screen is wrong:\n%s", p.ScreenText())
			}
			entries := app.history.Peek()
			if len(entries) != 1 || len(entries[0].Response.Events) != 5 {
				tt.Fatal("stopping lost the stream's history")
			}
			store := history.ForCollection(tt.TempDir(), "/sse")
			if _, err := store.Append(entries[0]); err != nil {
				tt.Fatal(err)
			}
			loaded, err := store.Load()
			if err != nil || len(loaded) != 1 || len(loaded[0].Response.Events) != 5 {
				tt.Fatalf("history didn't preserve event data and metadata: %v", err)
			}
			for i, got := range loaded[0].Response.Events {
				want := entries[0].Response.Events[i]
				if got.Event != want.Event || got.ID != want.ID || got.Title != want.Title || string(got.Body) != string(want.Body) || !got.At.Equal(want.At) {
					tt.Fatalf("history event %d = %+v, want %+v", i, got, want)
				}
			}
			app.openHistory(loaded[0])
			p.Advance(0)
			if !strings.Contains(p.ScreenText(), "HISTORY") || !strings.Contains(p.ScreenText(), "deployment") {
				tt.Fatalf("reopening history lost the event log:\n%s", p.ScreenText())
			}
		})
	}
}
