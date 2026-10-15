package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
	"github.com/gorilla/websocket"
)

func TestWebSocketConnectSendInspectDisconnectAndHistory(tt *testing.T) {
	for _, tc := range []struct {
		theme string
		width int
	}{{"galaxy", 130}, {"catppuccin-latte", 82}} {
		tt.Run(tc.theme, func(tt *testing.T) {
			frames := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				closed := make(chan struct{})
				type message struct {
					typ  int
					body []byte
				}
				echoes := make(chan message)
				go func() {
					defer close(closed)
					for {
						typ, body, err := conn.ReadMessage()
						if err != nil {
							return
						}
						echoes <- message{typ, body}
					}
				}()
				for {
					select {
					case message := <-echoes:
						if err := conn.WriteMessage(message.typ, message.body); err != nil {
							return
						}
					case frame := <-frames:
						if err := conn.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
							return
						}
					case <-closed:
						return
					}
				}
			}))
			defer server.Close()
			app := homeApp(tc.theme)
			app.sender = &client.WebSocket{HTTP: client.NewHTTP("posting-test", client.TLSSettings{})}
			req := model.WebSocketKind.New()
			req.Name, req.URL = "Deployment updates", strings.Replace(server.URL, "http", "ws", 1)
			req.Payload = model.WebSocket{Message: `{"service":"api","progress":25}`}
			app.openRequest(req)
			s := app.current()
			defer s.Cancel()
			updates := make(chan func(), 64)
			s.dispatch = func(fn func()) { updates <- fn }
			p := t.NewPilot(tt, app, tc.width, snapH)
			if !strings.Contains(p.TextOf(sendButtonID), "Connect") {
				tt.Fatal("no connect button")
			}
			p.Click(sendButtonID)
			drainUntil(tt, updates, func() bool { return s.live && s.streaming.Peek() == streamOpen })
			p.Advance(0)
			if len(s.response.Peek().Events) != 1 || !strings.Contains(p.ScreenText(), "WebSocket connected") {
				tt.Fatal("connect sent the draft or failed to display")
			}
			if strings.Contains(p.ScreenText(), "not connected") {
				tt.Fatal("clicking Connect also tried to send")
			}
			p.Click(sendButtonID)
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 3 })
			p.Advance(0)
			t.RequestFocus(messagesID)
			p.Advance(0)
			p.Press("enter")
			if !strings.Contains(p.TextOf("resp-body"), `"progress": 25`) {
				tt.Fatal("echo not shown")
			}
			p.Press("t")
			if _, ok := p.Bounds(responseJSONQueryID); !ok {
				tt.Fatal("no JSON inspector")
			}
			frames <- `{"service":"api","progress":80}`
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 4 })
			p.Advance(0) // No keyboard/mouse input accompanies this arrival.
			if !strings.Contains(p.ScreenText(), "1 new") || !strings.Contains(p.TextOf("resp-body"), "25") || strings.Contains(p.TextOf("resp-body"), "80") {
				tt.Fatalf("arrival moved the inspected payload:\n%s", p.ScreenText())
			}
			p.Press("escape")
			if s.phase.Peek() != exchangeSending {
				tt.Fatal("leaving inspector disconnected")
			}
			p.Press("f")
			if !strings.Contains(s.responseBody.GetText(), "80") {
				tt.Fatal("follow didn't select latest")
			}
			e := s.payloads[model.KindWebSocket].(*webSocketEditor)
			e.binary.Checked.Set(true)
			e.message.SetText("AP+A")
			p.Press("ctrl+j")
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 6 })
			p.Advance(0)
			p.Press("enter")
			if !strings.Contains(p.ScreenText(), "binary (base64)") || strings.TrimSpace(p.TextOf("resp-body")) != "AP+A" {
				tt.Fatalf("binary inspector wrong:\n%s", p.ScreenText())
			}
			p.Press("escape")
			p.Press("f")
			e.binary.Checked.Set(false)
			e.message.SetText("")
			p.Press("ctrl+j")
			drainUntil(tt, updates, func() bool { return len(s.response.Peek().Events) == 8 })
			p.Advance(0)
			p.Press("enter")
			if !strings.Contains(p.ScreenText(), "Empty message") {
				tt.Fatal("empty message lost")
			}
			p.Press("escape")
			p.Press("alt+j")
			drainUntil(tt, updates, func() bool { return s.phase.Peek() == exchangeDone })
			p.Advance(0)
			if s.responseStatus.Code != "1000" || strings.Contains(p.ScreenText(), "Streaming") {
				tt.Fatal("disconnect failed")
			}
			store := history.ForCollection(tt.TempDir(), "/websocket")
			if len(app.history.Peek()) != 1 {
				tt.Fatal("no history entry")
			}
			if _, err := store.Append(app.history.Peek()[0]); err != nil {
				tt.Fatal(err)
			}
			loaded, err := store.Load()
			if err != nil || len(loaded) != 1 || len(loaded[0].Response.Events) != 9 || loaded[0].Response.Events[5].FrameType != "binary" || string(loaded[0].Response.Events[5].Body) != string([]byte{0, 255, 128}) {
				tt.Fatal("history lost messages", err)
			}
			app.openHistory(loaded[0])
			p.Advance(0)
			if !strings.Contains(p.ScreenText(), "HISTORY") || !strings.Contains(p.ScreenText(), "disconnected: 1000") {
				tt.Fatalf("history not visible:\n%s", p.ScreenText())
			}
		})
	}
}
