package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/importing"
	"github.com/darrenburns/posting/v3/internal/importing/bruno"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestImportedAliasesReachSenderAndCurlWithRequestScope(t *testing.T) {
	source := t.TempDir()
	files := map[string]string{
		"bruno.json":               `{}`,
		"collection.bru":           "vars:pre-request {\n baseUrl: https://collection.test\n}\n",
		"folder/folder.bru":        "vars:pre-request {\n host: folder.test\n}\n",
		"folder/ping.bru":          "post {\n url: {{baseUrl}}/graphql\n body: graphql\n}\nvars:pre-request {\n host: request.test\n}\nheaders {\n X-Secret: {{secretAlias}}\n}\nbody:graphql {\n query Q($id: ID!) { user(id: $id) { name } }\n}\n",
		"environments/staging.bru": "vars {\n host: environment.test\n baseUrl: https://{{host}}\n secretAlias: Bearer {{token}}\n}\nvars:secret [token]\n",
	}
	for name, data := range files {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	imported, err := bruno.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	written, err := importing.Write(imported, dest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "staging.local.env"), []byte("token='${literal}$cash'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, written.Files[0]))
	if err != nil {
		t.Fatal(err)
	}
	req, err := collection.ParseRequest(data, written.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	app := New(Config{Environments: env.Source{}, Environment: env.Stack(dest, "staging")})
	app.openRequest(req)
	s := app.current()
	if s.Snapshot().VariableScope.Variables["host"] != "request.test" {
		t.Fatal("session dropped request scope")
	}
	if app.variableValuesPeek()["baseUrl"] != "https://request.test" {
		t.Fatal(app.variableValuesPeek())
	}
	app.sessionVars.Set(map[string]string{"host": "session.test"})
	command, err := app.exportCommand(true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "https://session.test/graphql") || !strings.Contains(command, "Bearer ${literal}$cash") {
		t.Fatal(command)
	}
	received := make(chan client.Call, 1)
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	app.sender = client.SenderFunc(func(_ context.Context, call client.Call) (*model.Response, error) {
		received <- call
		return fixedResponse(), nil
	})
	app.send()
	select {
	case call := <-received:
		resolved, err := model.Resolve(call.Request, call.Lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.URL != "https://session.test/graphql" {
			t.Fatal(resolved.URL)
		}
		if resolved.Headers[0].Value != "Bearer ${literal}$cash" {
			t.Fatal(resolved.Headers)
		}
		if !strings.Contains(resolved.Payload.(model.GraphQL).Query, "$id") {
			t.Fatal(resolved.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach sender")
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(2 * time.Second):
		t.Fatal("send did not finish")
	}
}

func TestInvalidImportedAliasStopsSendAndCurl(t *testing.T) {
	for _, template := range []string{"https://example.test/${MISSING_IMPORTED_TOKEN}", "${base}"} {
		app := New(Config{})
		req := model.NewRequest()
		req.URL = "${base}"
		req.VariableScope = &model.VariableScope{Variables: map[string]string{"base": template}}
		app.openRequest(req)
		app.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) {
			t.Error("invalid alias reached sender")
			return nil, nil
		})
		app.send()
		if app.current().err.Peek() == nil {
			t.Fatal("send did not show alias error")
		}
		if _, err := app.exportCommand(true); err == nil {
			t.Fatal("curl accepted invalid alias")
		}
	}
}

func TestInvalidAliasCancelsPreviousExchange(t *testing.T) {
	app := New(Config{})
	req := model.NewRequest()
	req.URL = "https://example.test"
	app.openRequest(req)
	s := app.current()
	started := make(chan context.Context, 1)
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	app.sender = client.SenderFunc(func(ctx context.Context, _ client.Call) (*model.Response, error) {
		started <- ctx
		<-ctx.Done()
		return nil, ctx.Err()
	})
	app.send()
	var ctx context.Context
	select {
	case ctx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sender did not start")
	}
	s.url.SetText("${base}")
	s.variableScope = &model.VariableScope{Variables: map[string]string{"base": "${base}"}}
	app.send()
	if ctx.Err() == nil || s.phase.Peek() != exchangeFailed || s.err.Peek() == nil {
		t.Fatalf("previous exchange wasn't cancelled with a visible error: phase=%v error=%v", s.phase.Peek(), s.err.Peek())
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled sender did not finish")
	}
	if s.phase.Peek() != exchangeFailed {
		t.Fatal("stale callback replaced validation failure")
	}
}
