package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/gorilla/websocket"
)

func socketRequest(server *httptest.Server) model.Request {
	r := model.WebSocketKind.New()
	r.URL = strings.Replace(server.URL, "http", "ws", 1)
	return r
}

func TestWebSocketInteractiveExchange(t *testing.T) {
	gotRequest := make(chan *http.Request, 1)
	pong := make(chan string, 1)
	closed := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest <- r
		upgrader := websocket.Upgrader{Subprotocols: []string{"events.v1"}, WriteBufferSize: 14}
		conn, err := upgrader.Upgrade(w, r, http.Header{"Set-Cookie": {"socket=connected; Path=/"}})
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		conn.SetPongHandler(func(data string) error { pong <- data; return nil })
		conn.WriteControl(websocket.PingMessage, []byte("alive"), time.Now().Add(time.Second))
		// Fragmented messages must appear as one event, even across UTF-8 bytes.
		writer, _ := conn.NextWriter(websocket.TextMessage)
		writer.Write([]byte("{\"city\":\"Montr\xc3"))
		writer.Write([]byte("\xa9al\"}"))
		writer.Close()
		for {
			typ, body, err := conn.ReadMessage()
			if err != nil {
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					closed <- closeErr.Code
				}
				return
			}
			if err := conn.WriteMessage(typ, body); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	req := socketRequest(server)
	req.URL += "/:team"
	req.PathParams = []model.KeyValue{{Name: "team", Value: "core", Enabled: true}}
	req.Query = []model.KeyValue{{Name: "key", Value: "a&b", Enabled: true}}
	req.Headers = []model.KeyValue{{Name: "Sec-WebSocket-Protocol", Value: "events.v1", Enabled: true}, {Name: "X-Ignore", Value: "no", Enabled: false}}
	req.Auth = model.Auth{Type: model.AuthBearer, Token: "${TOKEN}"}
	req.Payload = model.WebSocket{Message: "must not be sent on connect"}
	req.Options.TimeoutSeconds = .2
	h := NewHTTP("posting-test", TLSSettings{})
	socket := NewSocket()
	updates, done := make(chan *model.Response, 100), make(chan *model.Response, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		resp, err := (&WebSocket{HTTP: h}).Send(ctx, Call{Request: req, Variables: map[string]string{"TOKEN": "test-token", "NAME": "Ada"}, Socket: socket, OnUpdate: func(r *model.Response) { updates <- r }})
		if err != nil {
			t.Error(err)
		}
		done <- resp
	}()
	await := func(n int) *model.Response {
		t.Helper()
		for {
			select {
			case resp := <-updates:
				if len(resp.Events) >= n {
					return resp
				}
			case <-ctx.Done():
				t.Fatal("no live update")
			}
		}
	}
	first := await(2)
	if first.Header("Sec-Websocket-Protocol") != "events.v1" || string(first.Events[1].Body) != `{"city":"Montréal"}` {
		t.Fatalf("first response: %+v", first)
	}
	r := <-gotRequest
	if r.URL.RequestURI() != "/core?key=a%26b" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-Ignore") != "" || r.UserAgent() != "posting-test" {
		t.Fatalf("handshake = %s %v", r.URL, r.Header)
	}
	select {
	case got := <-pong:
		if got != "alive" {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal("no pong while idle")
	}
	time.Sleep(250 * time.Millisecond) // The UI's connect timeout must now be lifted.
	for i, message := range []model.WebSocket{{Message: `{"name":"${NAME}"}`}, {Message: "AP+A", Binary: true}, {}, {Binary: true}} {
		if err := socket.Send(message); err != nil {
			t.Fatal(err)
		}
		resp := await(4 + 2*i)
		sent, received := resp.Events[2+2*i], resp.Events[3+2*i]
		want, frame := []byte{}, "text"
		switch i {
		case 0:
			want = []byte(`{"name":"Ada"}`)
		case 1:
			want, frame = []byte{0, 255, 128}, "binary"
		case 3:
			frame = "binary"
		}
		if string(sent.Body) != string(want) || string(received.Body) != string(want) || sent.Kind != model.MessageSent || received.Kind != model.MessageReceived || received.FrameType != frame {
			t.Fatalf("exchange %d: %+v %+v", i, sent, received)
		}
	}
	if err := socket.Send(model.WebSocket{Binary: true, Message: "not base64!"}); err == nil {
		t.Fatal("invalid binary sent")
	}
	socket.Close()
	var resp *model.Response
	select {
	case resp = <-done:
	case <-ctx.Done():
		t.Fatal("disconnect blocked")
	}
	if resp == nil || len(resp.Events) != 11 || resp.WebSocket.Code != 1000 || resp.StreamError != "" {
		t.Fatalf("final = %+v", resp)
	}
	if <-closed != 1000 {
		t.Fatal("disconnect didn't send a normal close")
	}
	if len(first.Events) != 2 || first.WebSocket != nil {
		t.Fatal("a published snapshot was mutated")
	}
	if socket.TakesMessages() || socket.Send(model.WebSocket{}) == nil {
		t.Fatal("closed socket takes messages")
	}
	var transcript []model.MessageEvent
	if err := json.Unmarshal(resp.Body, &transcript); err != nil || !reflect.DeepEqual(transcript[5].Body, []byte{0, 255, 128}) {
		t.Fatal("export lost binary bytes", err)
	}
}

func TestWebSocketCloseTimeoutAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		closeCode int
		data      []byte
		wait      bool
		want      string
	}{
		{"normal", 1000, []byte("hello"), false, ""},
		{"policy", 1008, []byte("hello"), false, "policy"},
		{"dropped", 0, []byte("partial"), false, "unexpected EOF"},
		{"timeout", 0, []byte("partial"), true, "timed out"},
		{"oversized", 0, []byte(strings.Repeat("x", maxWebSocketMessageBytes+1)), false, "read limit"},
		{"invalid UTF-8", 0, []byte{255}, false, "invalid UTF-8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				conn.WriteMessage(websocket.TextMessage, tc.data)
				if tc.closeCode != 0 {
					conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(tc.closeCode, "policy"), time.Now().Add(time.Second))
				}
				if tc.wait {
					conn.ReadMessage()
				}
			}))
			defer server.Close()
			req := socketRequest(server)
			req.Options.TimeoutSeconds = .15
			resp, err := (&WebSocket{HTTP: NewHTTP("", TLSSettings{})}).Send(context.Background(), Call{Request: req})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" && resp.StreamError != "" || tc.want != "" && !strings.Contains(resp.StreamError, tc.want) {
				t.Fatalf("stream error = %q, want %q", resp.StreamError, tc.want)
			}
			if len(tc.data) < maxWebSocketMessageBytes && tc.name != "invalid UTF-8" && string(resp.Events[1].Body) != string(tc.data) {
				t.Fatal("lost partial data")
			}
			if tc.closeCode != 0 && (resp.WebSocket.Code != tc.closeCode || resp.WebSocket.Reason != "policy") {
				t.Fatal("lost close status")
			}
		})
	}
}

func TestWebSocketHandshakeCancellationAndTLS(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (&WebSocket{HTTP: NewHTTP("", TLSSettings{})}).Send(ctx, Call{Request: socketRequest(server)})
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelling a stalled upgrade did not close it")
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, body, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(websocket.TextMessage, body)
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, "finished"), time.Now().Add(time.Second))
	}))
	defer tlsServer.Close()
	req := socketRequest(tlsServer)
	req.Payload = model.WebSocket{Message: "headless"}
	w := &WebSocket{HTTP: NewHTTP("", TLSSettings{})}
	if _, err := w.Send(context.Background(), Call{Request: req}); err == nil {
		t.Fatal("untrusted TLS accepted")
	}
	req.Options.VerifySSL = false
	resp, err := w.Send(context.Background(), Call{Request: req})
	if err != nil || resp.WebSocket.Code != 1000 || len(resp.Events) != 4 || string(resp.Events[2].Body) != "headless" {
		t.Fatalf("headless TLS exchange = %+v, %v", resp, err)
	}
}
