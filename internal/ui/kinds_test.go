package ui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

func TestEveryKindIsWired(tt *testing.T) {
	hotkeys := map[string]string{}
	for method, key := range methodHotkeys {
		hotkeys[key] = string(method)
	}
	for _, kind := range model.Kinds {
		tt.Run(string(kind.ID), func(tt *testing.T) {
			view, ok := kindViews[kind.ID]
			if !ok {
				tt.Fatal("no kindView")
			}
			if view.urlPlaceholder == "" {
				tt.Error("no URL placeholder")
			}
			if kind != model.HTTPKind {
				if view.color == nil || view.newEditor == nil {
					tt.Fatal("a kind with a payload needs a colour and an editor")
				}
				if view.hotkey == "" {
					tt.Error("no hotkey in the method selector")
				} else if other, taken := hotkeys[view.hotkey]; taken {
					tt.Errorf("hotkey %q is already %s's", view.hotkey, other)
				}
				hotkeys[view.hotkey] = kind.Label
			}

			app := testApp()
			s := app.current()
			want := kind.Example()
			s.Load(want)
			if got := s.Snapshot(); !reflect.DeepEqual(got, want) {
				tt.Fatalf("Load then Snapshot changed the request:\n got %+v\nwant %+v", got, want)
			}
			if got := s.requestTabList()[0].key; got != s.requestTab.Peek() {
				tt.Errorf("loading a %s request shows tab %q, want its first tab %q", kind.Label, s.requestTab.Peek(), got)
			}
		})
	}
}

func TestKindColoursDifferFromMethodColours(tt *testing.T) {
	for _, name := range t.ThemeNames() {
		theme, _ := t.GetTheme(name)
		for _, kind := range model.Kinds {
			if kind == model.HTTPKind {
				continue
			}
			color := requestColor(theme, kind.New())
			for _, method := range model.Methods {
				if color == methodColor(theme, method) {
					tt.Errorf("%s: %s requests are coloured %s, like %s's %s", name, kind.Label, color.Hex(), method, methodColor(theme, method).Hex())
				}
			}
		}
	}
}

func graphQLRequest(tt *testing.T) model.Request {
	tt.Helper()
	r := model.GraphQLKind.Example()
	r.Payload = model.GraphQL{
		Query:         "query User($id: ID!) {\n  user(id: $id) {\n    name\n    email # ${SHOW_EMAIL}\n  }\n}\n",
		Variables:     "{\n  \"id\": \"${USER_ID}\"\n}\n",
		OperationName: "User",
	}
	return r
}

func TestRequestTabsFollowTheKind(tt *testing.T) {
	app := testApp()
	app.openRequest(graphQLRequest(tt))
	s := app.current()
	var labels []string
	for _, tab := range s.requestTabs().Tabs {
		labels = append(labels, tab.Label)
	}
	want := []string{"Query", "Variables", "Headers", "Path", "Params", "Auth", "Info", "Options"}
	if !reflect.DeepEqual(labels, want) {
		tt.Fatalf("GraphQL tabs = %v, want %v", labels, want)
	}
	if s.requestTab.Peek() != "gql-query" {
		tt.Fatalf("a GraphQL request opens on %q, want its Query tab", s.requestTab.Peek())
	}

	s.requestTab.Set("auth")
	app.setMethod(model.MethodPost)
	if s.requestTab.Peek() != "headers" || !strings.Contains(tabLabels(s), "Body") {
		tt.Fatalf("after switching to HTTP: tab %q, tabs %s", s.requestTab.Peek(), tabLabels(s))
	}
}

func tabLabels(s *Session) string {
	var labels []string
	for _, tab := range s.requestTabs().Tabs {
		labels = append(labels, tab.Label)
	}
	return strings.Join(labels, ",")
}

func TestSwitchingKindKeepsTheEnvelopeAndEachKindsEdits(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	s := app.current()
	before := s.Snapshot()

	app.setKind(model.KindGraphQL)
	if !s.dirty.Peek() {
		tt.Error("switching kind is an edit")
	}
	s.payloads[model.KindGraphQL].(*graphQLEditor).query.SetText("{ whoami }")
	gql := s.Snapshot()
	if gql.Kind() != model.GraphQLKind || gql.URL != before.URL || !reflect.DeepEqual(gql.Headers, before.Headers) || gql.Auth != before.Auth {
		tt.Fatalf("GraphQL snapshot lost the envelope: %+v", gql)
	}
	if gql.Body.Type != model.BodyNone || gql.Method != model.MethodGet {
		tt.Fatalf("GraphQL snapshot carries HTTP's method %q and body %+v", gql.Method, gql.Body)
	}

	app.setMethod(model.MethodPost)
	if got := s.Snapshot(); !reflect.DeepEqual(got, before) {
		tt.Fatalf("back to HTTP, the request isn't what it was:\n got %+v\nwant %+v", got, before)
	}
	app.setKind(model.KindGraphQL)
	if got := s.Snapshot().Payload; got != (model.GraphQL{Query: "{ whoami }"}) {
		tt.Fatalf("the GraphQL query was lost across kinds: %+v", got)
	}
}

func TestMethodSelectorHotkeyPicksGraphQL(tt *testing.T) {
	app := testApp()
	s := app.current()
	var pick func()
	for _, bind := range (methodSelector{app: app}).Keybinds() {
		if bind.Key == "q" {
			pick = bind.Action
		}
	}
	if pick == nil {
		tt.Fatal("the method selector doesn't bind q")
	}
	pick()
	if s.kind.Peek() != model.KindGraphQL {
		tt.Fatalf("q made the request %s", s.kind.Peek())
	}
}

func TestTreeSearchFindsGraphQLByBadge(tt *testing.T) {
	gql := graphQLRequest(tt)
	gql.Name = "Current user"
	get := sampleRequest(tt, "Get user")
	for term, want := range map[string]bool{"gql": true, "graph": true, "get": false} {
		if got := parseTreeQuery(term).matches(treeItem{Request: &gql}); got != want {
			tt.Errorf("%q matches a GraphQL request: %v, want %v", term, got, want)
		}
	}
	if !parseTreeQuery("get").matches(treeItem{Request: &get}) {
		tt.Error("method search no longer finds GET requests")
	}
}

func TestCopyGraphQLAsCurlSendsTheLoweredRequest(tt *testing.T) {
	app := testApp()
	app.openRequest(graphQLRequest(tt))
	command, err := app.curlCommand(true)
	if err != nil {
		tt.Fatal(err)
	}
	for _, want := range []string{"http://localhost:8000/graphql", "--data-raw '{\"query\":", `"operationName":"User"`, "Accept: application/graphql-response+json", "Bearer dev-token-123"} {
		if !strings.Contains(command, want) {
			tt.Errorf("curl command lacks %q:\n%s", want, command)
		}
	}
}

// graphQLApp is the test app with a GraphQL request in the collection's
// users folder, open in the current tab.
func graphQLApp(tt *testing.T) *App {
	tt.Helper()
	collection := model.SampleCollection()
	req := graphQLRequest(tt)
	for _, folder := range collection.Children {
		if folder.Name == "users" {
			folder.Requests = append(folder.Requests, req)
		}
	}
	collection.Sort()
	app := New(Config{
		Version:      "3.0.0-dev",
		Collection:   collection,
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
	})
	app.openRequest(req)
	return app
}

func TestSnapshotGraphQL(tt *testing.T) {
	app := graphQLApp(tt)
	t.AssertSnapshotNamed(tt, "GraphQL_query", app, snapW, snapH, "A GraphQL request from the collection: GQL badges, the Query tab with its operation row and highlighted document")

	app.current().requestTab.Set("gql-variables")
	t.AssertSnapshotNamed(tt, "GraphQL_variables", app, snapW, snapH, "The GraphQL Variables tab: JSON with a ${USER_ID} reference")

	app.openMethodMenu()
	t.AssertSnapshotNamed(tt, "GraphQL_method_menu", app, snapW, snapH, "The method menu: HTTP methods, a separator, then GraphQL on q")
}

func TestSnapshotGraphQLResponseWithErrors(tt *testing.T) {
	app := graphQLApp(tt)
	app.sidebarTab.Set("history")
	resp := fixedResponse()
	resp.Method = model.MethodPost
	resp.URL = "http://localhost:8000/graphql"
	resp.Body = []byte(`{"data": {"user": null}, "errors": [{"message": "User 42 not found", "path": ["user"]}]}`)
	entry := model.HistoryEntry{ID: 1, Request: graphQLRequest(tt), Response: resp, SentAt: time.Date(2026, 9, 28, 14, 30, 0, 0, time.UTC)}
	app.history.Set([]model.HistoryEntry{entry})
	app.historyList.SetItems([]model.HistoryEntry{entry})
	app.openHistory(entry)
	t.AssertSnapshot(tt, app, snapW, snapH, "A GraphQL 200 with errors: warning status chip, response title and history row")
}

func TestSendingGraphQLReadsItsStatusAndKeepsItsKindInHistory(tt *testing.T) {
	app := testApp()
	app.openRequest(graphQLRequest(tt))
	s := app.current()
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	app.sender = client.SenderFunc(func(_ context.Context, call client.Call) (*model.Response, error) {
		if call.Request.Kind() != model.GraphQLKind {
			return nil, fmt.Errorf("sender got a %s request", call.Request.Kind().Label)
		}
		resp := fixedResponse()
		resp.Body = []byte(`{"errors": [{"message": "a"}, {"message": "b"}]}`)
		return resp, nil
	})
	app.send()
	(<-updates)()
	if s.err.Peek() != nil {
		tt.Fatal(s.err.Peek())
	}
	if got := s.status(); got != (model.Status{Code: "200", Text: "2 errors", Class: model.StatusClassWarning}) {
		tt.Fatalf("status = %+v", got)
	}
	history := app.history.Peek()
	if len(history) != 1 || history[0].Request.Payload != graphQLRequest(tt).Payload {
		tt.Fatalf("history keeps the GraphQL request as edited, got %+v", history)
	}
}
