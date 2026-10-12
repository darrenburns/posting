package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestCollectionAgainstDemo(t *testing.T) {
	server, err := newServer()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	sender := client.NewGRPC("posting-demo-test", client.TLSSettings{}, "collection")
	load := func(t *testing.T, file string) model.Request {
		t.Helper()
		path := filepath.Join("collection", file+".posting.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		req, err := collection.ParseRequest(data, path)
		if err != nil {
			t.Fatal(err)
		}
		req.URL = "grpc://" + listener.Addr().String()
		return req
	}
	check := func(t *testing.T, response *model.Response, want string) {
		t.Helper()
		if response.StatusCode != 0 {
			t.Fatalf("status = %d: %s", response.StatusCode, response.Body)
		}
		var gotJSON, wantJSON any
		if err := json.Unmarshal(response.Body, &gotJSON); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Fatalf("got %s; want %s", response.Body, want)
		}
	}
	for _, c := range []struct{ file, want string }{
		{"01-unary", `{"text":"Hello, Ada!","sequence":1}`},
		{"02-server-stream", `[{"text":"Hello, Grace!","sequence":1},{"text":"Bonjour, Grace!","sequence":2},{"text":"Hola, Grace!","sequence":3}]`},
		{"03-client-stream", `{"names":["Ada","Grace","Linus"],"count":3}`},
		{"04-bidi-stream", `[{"text":"HELLO FROM POSTING!","sequence":1}]`},
	} {
		t.Run(c.file, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			response, err := sender.Send(ctx, client.Call{Request: load(t, c.file)})
			if err != nil {
				t.Fatal(err)
			}
			check(t, response, c.want)
		})
	}
	t.Run("interactive-bidi", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		stream := client.NewStream()
		updates := make(chan *model.Response, 32)
		done := make(chan *model.Response, 1)
		failed := make(chan error, 1)
		request := load(t, "04-bidi-stream")
		go func() {
			response, err := sender.Send(ctx, client.Call{
				Request: request, Stream: stream,
				OnUpdate: func(response *model.Response) { updates <- response },
			})
			if err != nil {
				failed <- err
				return
			}
			done <- response
		}()
		waitFor := func(count int, want string) {
			t.Helper()
			for {
				select {
				case response := <-updates:
					var messages []json.RawMessage
					if json.Unmarshal(response.Body, &messages) == nil && len(messages) == count {
						check(t, response, want)
						if !stream.TakesMessages() {
							t.Fatal("stream ended before the client half-closed")
						}
						return
					}
				case response := <-done:
					t.Fatalf("call ended before half-close: %s", response.Body)
				case err := <-failed:
					t.Fatal(err)
				case <-ctx.Done():
					t.Fatal("reply did not arrive while the stream was open")
				}
			}
		}
		waitFor(1, `[{"text":"HELLO FROM POSTING!","sequence":1}]`)
		if err := stream.Send(`{"text":"Still on the same stream"}`); err != nil {
			t.Fatal(err)
		}
		want := `[{"text":"HELLO FROM POSTING!","sequence":1},{"text":"STILL ON THE SAME STREAM","sequence":2}]`
		waitFor(2, want)
		stream.Close()
		select {
		case response := <-done:
			check(t, response, want)
			for _, trailer := range response.Trailers {
				if trailer.Name == "x-messages-sent" && trailer.Value == "2" {
					return
				}
			}
			t.Fatalf("missing message-count trailer: %v", response.Trailers)
		case err := <-failed:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("half-close did not end the call")
		}
	})
}
