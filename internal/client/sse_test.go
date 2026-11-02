package client

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestHTTPSSEFraming(t *testing.T) {
	// Exercise CR, LF and CRLF, including an ID-only block, empty data,
	// multiline data, a NUL-containing ID, and EOF before a dispatch boundary.
	const wire = "\uFEFF: heartbeat\r\nevent: discarded\r\nid: first\r\n\r\n" +
		"event: update\rdata: {\rdata: \"count\": 7}\r\r" +
		"data:  café:42\nid: bad\x00id\nretry: 1000\nunknown: ignored\n\n" +
		"id\ndata\n\n" + "data:\ndata:\n\n" +
		"event: tail\ndata: never dispatched\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "Text/Event-Stream; charset=utf-8")
		io.WriteString(w, wire)
	}))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
	if string(resp.Body) != wire || resp.StreamError != "" || resp.StatusCode != 200 {
		t.Fatalf("response = %+v", resp)
	}
	type event struct{ name, id, data, contentType string }
	var got []event
	for _, e := range resp.Events {
		if e.Kind == model.MessageReceived {
			got = append(got, event{e.Event, e.ID, string(e.Body), e.ContentType})
		}
	}
	want := []event{
		{"update", "first", "{\n\"count\": 7}", "application/json"},
		{"message", "first", " café:42", "text/plain"},
		{"message", "", "", "text/plain"},
		{"message", "", "\n", "text/plain"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	if resp.Events[len(resp.Events)-1].Title != "Stream ended" {
		t.Fatalf("missing end event: %+v", resp.Events)
	}
}

func TestHTTPSSEStreamsPastTimeoutAndStopsWithoutLosingEvents(t *testing.T) {
	for _, encoding := range []string{"identity", "gzip", "deflate", "raw-deflate"} {
		t.Run(encoding, func(t *testing.T) {
			next := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if r.Method != "POST" || string(body) != `{"prompt":"hello"}` || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("wrong request: %s, %s, auth %q", r.Method, body, r.Header.Get("Authorization"))
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Content-Encoding", strings.TrimPrefix(encoding, "raw-"))
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "sse"})
				w.(http.Flusher).Flush()
				var writer io.Writer = w
				var flush func() error
				switch encoding {
				case "gzip":
					z := gzip.NewWriter(w)
					defer z.Close()
					writer, flush = z, z.Flush
				case "deflate":
					z := zlib.NewWriter(w)
					defer z.Close()
					writer, flush = z, z.Flush
				case "raw-deflate":
					z, _ := flate.NewWriter(w, flate.DefaultCompression)
					defer z.Close()
					writer, flush = z, z.Flush
				}
				write := func(text string) {
					io.WriteString(writer, text)
					if flush != nil {
						flush()
					}
					w.(http.Flusher).Flush()
				}
				// No following byte: a CR-only separator must dispatch now.
				write("id: 17\revent: token\rdata: {\"text\":\"hello\"}\r\r")
				select {
				case <-next:
				case <-r.Context().Done():
					return
				}
				write("\ndata: second\n\n") // split CRLF across network writes
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := model.NewRequest()
			req.Method, req.URL = model.MethodPost, server.URL
			req.Options.TimeoutSeconds = 0.25
			req.Auth = model.Auth{Type: model.AuthBearer, Token: "${TOKEN}"}
			req.Body = model.Body{Type: model.BodyRaw, Raw: `{"prompt":"hello"}`, ContentType: "application/json"}
			updates, finished := make(chan *model.Response, 16), make(chan *model.Response, 1)
			go func() {
				resp, err := NewHTTP("", TLSSettings{}).Send(ctx, Call{
					Request: req, Variables: map[string]string{"TOKEN": "test-token"},
					OnUpdate: func(resp *model.Response) { updates <- resp },
				})
				if err != nil {
					t.Errorf("send: %v", err)
				}
				finished <- resp
			}()
			await := func(count int) *model.Response {
				t.Helper()
				deadline := time.After(3 * time.Second)
				for {
					select {
					case resp := <-updates:
						if len(resp.Events) == count {
							return resp
						}
					case resp := <-finished:
						t.Fatalf("stream ended before update: %+v", resp)
					case <-deadline:
						t.Fatal("no live SSE update before the server closed")
					}
				}
			}
			connected := await(1)
			if len(connected.Cookies) != 1 || connected.Cookies[0].Value != "sse" {
				t.Fatalf("headers/cookies aren't available on connect: %+v", connected)
			}
			first := await(2)
			if first.Events[1].ID != "17" || string(first.Events[1].Body) != `{"text":"hello"}` {
				t.Fatalf("first event: %+v", first.Events[1])
			}
			time.Sleep(300 * time.Millisecond) // beyond the ordinary request budget
			close(next)
			second := await(3)
			if second.Events[2].ID != "17" || string(second.Events[2].Body) != "second" || len(first.Events) != 2 {
				t.Fatalf("later event or earlier snapshot changed: %+v", second)
			}
			cancel()
			select {
			case resp := <-finished:
				if resp == nil || resp.StreamError != "" || len(resp.Events) != 4 || resp.Events[3].Title != "Stream stopped" || !strings.Contains(string(resp.Body), "data: second\n\n") {
					t.Fatalf("stop lost the response: %+v", resp)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stop didn't close the stream")
			}
		})
	}
}

func TestHTTPSSEFailureKeepsPartialResponse(t *testing.T) {
	for _, tc := range []struct {
		name, body, failure string
		hold, truncated     bool
	}{
		{"headless timeout", "data: kept\n\n", "timed out", true, false},
		{"truncated stream", "data: kept\n\ndata: incomplete", "unexpected EOF", false, true},
		{"oversized line", "data: kept\n\ndata:" + strings.Repeat("x", 1<<20), "SSE line exceeded", false, false},
		{"oversized multiline event", "data: kept\n\n" + strings.Repeat("data: a\n", (1<<20)/6+1), "SSE event exceeded", false, false},
		{"event count", strings.Repeat("data: kept\n\n", 10_001), "SSE log reached", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if tc.truncated {
					w.Header().Set("Content-Length", "1000")
				}
				io.WriteString(w, tc.body)
				w.(http.Flusher).Flush()
				if tc.hold {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL = server.URL
			if tc.hold {
				req.Options.TimeoutSeconds = 0.1
			}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if !strings.Contains(resp.StreamError, tc.failure) || len(resp.Events) < 3 || string(resp.Events[1].Body) != "kept" || model.StatusOf(req, resp).Class != model.StatusClassWarning {
				t.Fatalf("partial response or error missing: error %q, events %d, status %+v", resp.StreamError, len(resp.Events), model.StatusOf(req, resp))
			}
			if last := resp.Events[len(resp.Events)-1]; last.Kind != model.MessageError || !strings.Contains(last.Title, tc.failure) {
				t.Errorf("terminal event = %+v", last)
			}
		})
	}
}

func TestHTTPSSEDetectionDoesNotChangeOtherResponses(t *testing.T) {
	for _, tc := range []struct{ method, contentType string }{{"GET", "text/plain"}, {"HEAD", "text/event-stream"}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				io.WriteString(w, "data: ordinary body\n\n")
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL, req.Method = server.URL, model.Method(tc.method)
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if len(resp.Events) != 0 || tc.method == "GET" && string(resp.Body) != "data: ordinary body\n\n" {
				t.Fatalf("ordinary response was treated as SSE: %+v", resp)
			}
		})
	}
}
