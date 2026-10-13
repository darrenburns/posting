package ui

import (
	"context"
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
)

type completionDescriber struct{ calls chan client.Call }

func (d completionDescriber) Describe(_ context.Context, call client.Call) (client.Schema, error) {
	d.calls <- call
	return client.Schema{}, nil
}

func receiveCompletion[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for request or discovery completion")
	}
	var zero T
	return zero
}

func TestGRPCCompletionOnlyRefreshesUnchangedDiscoveryContext(t *testing.T) {
	for _, tc := range []struct {
		name    string
		capture string
		edit    func(*App, *Session, *grpcEditor)
		refresh bool
	}{
		{name: "unchanged", refresh: true},
		{name: "message only", refresh: true, edit: func(_ *App, _ *Session, e *grpcEditor) { e.message.SetText(`{"text":"new message"}`) }},
		{name: "unrelated response capture", capture: "OTHER", refresh: true},
		{name: "endpoint", edit: func(_ *App, s *Session, _ *grpcEditor) { s.url.SetText("second.test:50051") }},
		{name: "session endpoint", edit: func(a *App, _ *Session, _ *grpcEditor) {
			a.sessionVars.Set(map[string]string{"ENDPOINT": "second.test:50051"})
		}},
		{name: "environment endpoint", edit: func(a *App, _ *Session, _ *grpcEditor) {
			a.env.active.Set(model.Environment{Variables: []model.Variable{{Name: "ENDPOINT", Value: "second.test:50051"}, {Name: "TOKEN", Value: "original-token"}}})
		}},
		{name: "response capture endpoint", capture: "ENDPOINT"},
		{name: "bearer token", edit: func(_ *App, s *Session, _ *grpcEditor) { s.token.SetText("changed-token") }},
		{name: "session credentials", edit: func(a *App, _ *Session, _ *grpcEditor) {
			a.sessionVars.Set(map[string]string{"TOKEN": "changed-token"})
		}},
		{name: "metadata", edit: func(_ *App, s *Session, _ *grpcEditor) {
			s.headers.Load([]model.KeyValue{{Name: "x-tenant", Value: "other", Enabled: true}})
		}},
		{name: "authority", edit: func(_ *App, _ *Session, e *grpcEditor) { e.authority.SetText("other.test") }},
		{name: "TLS verification", edit: func(_ *App, s *Session, _ *grpcEditor) { s.verifySSL.SetChecked(false) }},
		{name: "kind", edit: func(_ *App, s *Session, _ *grpcEditor) { s.kind.Set(model.KindHTTP) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp()
			app.env.active.Set(model.Environment{Variables: []model.Variable{{Name: "ENDPOINT", Value: "first.test:50051"}, {Name: "TOKEN", Value: "original-token"}}})
			req := grpcRequest()
			req.URL = "${ENDPOINT}"
			req.Auth.Token = "${TOKEN}"
			req.Payload = model.GRPC{Method: "test.Service/Call", Message: `{"id":"${$uuid}"}`}
			if tc.capture != "" {
				req.Captures = map[string]string{tc.capture: "endpoint"}
			}
			app.openRequest(req)
			s := app.current()
			e := s.payloads[model.KindGRPC].(*grpcEditor)
			updates := make(chan func(), 8)
			s.dispatch = func(fn func()) { updates <- fn }
			described := make(chan client.Call, 8)
			app.describer = completionDescriber{described}
			started := make(chan client.Call, 1)
			finish := make(chan struct{})
			app.sender = client.SenderFunc(func(ctx context.Context, call client.Call) (*model.Response, error) {
				started <- call
				select {
				case <-finish:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return &model.Response{Proto: "gRPC", URL: "first.test:50051", Body: []byte(`{"endpoint":"second.test:50051"}`)}, nil
			})
			t.Cleanup(func() {
				s.Cancel()
				if e.cancel != nil {
					e.cancel()
				}
			})
			app.start(s, nil)
			sent := receiveCompletion(t, started)
			if tc.edit != nil {
				tc.edit(app, s, e)
			}
			close(finish)
			receiveCompletion(t, updates)()
			if !tc.refresh {
				if got := e.catalog.Peek(); got.phase != catalogIdle {
					t.Fatalf("completion started discovery for changed context: %+v", got)
				}
				if s.kind.Peek() == model.KindGRPC {
					e.describe(app, true)
					explicit := receiveCompletion(t, described)
					expected := s.Snapshot()
					if !reflect.DeepEqual(explicit.Request, expected) {
						t.Fatalf("explicit refresh did not use current editor: %+v", explicit.Request)
					}
					receiveCompletion(t, updates)()
				}
				return
			}
			got := receiveCompletion(t, described)
			if !reflect.DeepEqual(got.Request, sent.Request) || !reflect.DeepEqual(got.Variables, sent.Variables) {
				t.Fatalf("completion changed the original discovery context\nrequest: %+v\nvariables: %+v\nwant request: %+v\nwant variables: %+v", got.Request, got.Variables, sent.Request, sent.Variables)
			}
			receiveCompletion(t, updates)()
			if e.catalog.Peek().phase != catalogReady {
				t.Fatalf("discovery result did not arrive: %+v", e.catalog.Peek())
			}
		})
	}
}

func TestGRPCStartRetainsDeferredVariablesForStreamMessages(t *testing.T) {
	app := testApp()
	alias := "${WORD}/${HOST_ONLY}"
	t.Setenv("HOST_ONLY", "original-host")
	app.env.active.Set(model.Environment{Variables: []model.Variable{{Name: "ALIAS", Template: &alias}, {Name: "WORD", Value: "original-word"}}})
	req := grpcRequest()
	req.Payload = model.GRPC{Method: "test.Service/Call", Message: `{"id":"${$uuid}"}`}
	req.VariableScope = &model.VariableScope{Variables: map[string]string{"LATE": "${ALIAS}/${$uuid}"}}
	app.openRequest(req)
	s := app.current()
	updates := make(chan func(), 8)
	s.dispatch = func(fn func()) { updates <- fn }
	started := make(chan client.Call, 1)
	app.sender = client.SenderFunc(func(ctx context.Context, call client.Call) (*model.Response, error) {
		started <- call
		<-ctx.Done()
		return nil, ctx.Err()
	})
	t.Cleanup(s.Cancel)
	app.start(s, client.NewStream())
	call := receiveCompletion(t, started)
	before := maps.Clone(call.Variables)
	alias = "changed-template"
	t.Setenv("HOST_ONLY", "changed-host")
	app.sessionVars.Set(map[string]string{"WORD": "changed-word"})
	s.variableScope.Variables["LATE"] = "changed-scope"
	if call.ResolveBody == nil {
		t.Fatal("prepared call lost its deferred variable resolver")
	}
	got, err := call.ResolveBody("${LATE}")
	want := "original-word/original-host/" + before["$uuid"]
	if err != nil || got != want {
		t.Fatalf("late message = %q, %v; want %q", got, err, want)
	}
	s.Cancel()
	receiveCompletion(t, updates)()
}
