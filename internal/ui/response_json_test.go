package ui

import (
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestResponseJSONTree(tt *testing.T) {
	const body = `{"users":[{"id":9007199254740993,"name":"Ada"},{"id":2,"name":"Grace"}],"page":1}`
	app, s := bodyApp(tt, "application/json", body)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body")
	p.Press("g", "/")
	p.Type("Ada")
	p.Press("up")
	searchBounds, _ := p.Bounds(responseSearchID)
	textCursor := s.responseBody.CursorIndex.Peek()
	p.Click("resp-body-json")
	if p.FocusedID() != "resp-body" || !strings.Contains(p.TextOf("resp-body"), "9007199254740993") {
		tt.Fatalf("JSON tree did not open with focus:\n%s", p.ScreenText())
	}
	if bounds, ok := p.Bounds(responseJSONQueryID); !ok || bounds.Y != searchBounds.Y {
		tt.Fatalf("JSONPath bar is on row %d, want Find's row %d", bounds.Y, searchBounds.Y)
	}
	p.AssertSnapshot("tree", "JSON tree with a JSONPath input, syntax-coloured values and a switch back to the text view")
	p.Press("home", "left")
	if strings.Contains(p.TextOf("resp-body"), "Ada") {
		tt.Fatal("collapsing the root did not hide its children")
	}
	p.Press("right", "ctrl+f")
	if p.FocusedID() != responseJSONQueryID {
		tt.Fatal("ctrl+f should focus JSONPath in tree mode")
	}
	p.Type("$.users[1].name")
	if got := strings.TrimSpace(p.TextOf(responseJSONQueryID + "-count")); got != "1 match" {
		tt.Fatalf("query count = %q", got)
	}
	if got := p.TextOf("resp-body"); !strings.Contains(got, "Grace") || strings.Contains(got, "Ada") || !strings.Contains(got, "[1]") {
		tt.Fatalf("filter lost original indices or included a non-match:\n%s", got)
	}
	p.Press("enter", "end", "y")
	if got := p.Clipboard(); got != `"Grace"` {
		tt.Fatalf("copy value = %q", got)
	}
	p.Press("Y")
	if got := p.Clipboard(); got != "$['users'][1]['name']" {
		tt.Fatalf("copy path = %q", got)
	}
	app.toasts.Clear() // The copy notifications overlay the bottom search bar.
	if got := strings.TrimSpace(p.TextOf("resp-json-path")); got != "$['users'][1]['name']" {
		tt.Fatalf("cursor path = %q", got)
	}
	p.Press("/", "end")
	p.Type("[")
	if !strings.Contains(p.TextOf(responseJSONQueryID+"-error"), "Invalid JSONPath") || !strings.Contains(p.TextOf("resp-body"), "Grace") {
		tt.Fatal("an invalid query should explain that the last valid results remain")
	}
	if bounds, _ := p.Bounds(responseJSONQueryID); bounds.Y != searchBounds.Y {
		tt.Fatal("a query error moved the JSONPath bar away from Find's row")
	}
	p.Press("ctrl+a")
	p.Type("$.missing")
	if !strings.Contains(p.ScreenText(), "No JSONPath matches") || strings.TrimSpace(p.TextOf(responseJSONQueryID+"-count")) != "0 matches" {
		tt.Fatalf("an empty result should be explicit:\n%s", p.ScreenText())
	}
	p.Press("enter", "y")
	if p.Clipboard() != "$['users'][1]['name']" {
		tt.Fatal("copy with no results should leave the clipboard alone")
	}
	p.Press("escape", "/")
	p.Type("$.users[*].id")
	if got := strings.TrimSpace(p.TextOf(responseJSONQueryID + "-count")); got != "2 matches" {
		tt.Fatalf("query count = %q", got)
	}
	p.Press("enter", "home", "down", "down", "down", "y")
	if got := p.Clipboard(); got != "9007199254740993" {
		tt.Fatalf("copy rounded the JSON number: %q", got)
	}
	p.Press("t")
	if s.responseBody.CursorIndex.Peek() != textCursor || s.responseBody.GetSelectedText() != "Ada" || s.responseBody.SearchQuery.Peek() != "Ada" {
		tt.Fatal("switching views changed the text cursor, selection or search")
	}
	p.Press("y")
	if got := p.Clipboard(); got != "Ada" {
		tt.Fatalf("text view copy = %q", got)
	}
	p.Press("t", "/")
	p.Type("$.users[1].name")
	p.Resize(82, snapH)
	t.SetTheme("catppuccin-latte")
	app.toasts.Clear()
	p.AssertSnapshot("filtered_light", "Narrow light-theme JSONPath results retain ancestors and the original array index")
	p.Click("resp-body-text")
	if p.FocusedID() != "resp-body" || !strings.Contains(p.TextOf("resp-body"), `"users": [`) {
		tt.Fatal("clicking Text view should restore the focused text body")
	}
}

func TestResponseJSONReplacementAndSessions(tt *testing.T) {
	app, first := bodyApp(tt, "application/problem+json", `{"old":"first"}`)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body-json")
	p.Press("/")
	p.Type("$.old")
	p.Press("enter")
	second := app.openSession(sampleRequest(tt, "Create user"))
	second.showResponse(fixedResponse(), nil)
	second.phase.Set(exchangeDone)
	t.RequestFocus("resp-body")
	p.Advance(0)
	if _, ok := p.Bounds(responseJSONQueryID); ok {
		tt.Fatal("a new request tab inherited the tree view")
	}
	p.Press("alt+left")
	if app.current() != first || !strings.Contains(p.TextOf(responseJSONQueryID), "$.old") {
		tt.Fatal("returning to a session lost its JSON query")
	}
	resp := fixedResponse()
	resp.Body = []byte(`{"new":"replacement"}`)
	first.showResponse(resp, nil)
	p.Advance(0)
	if got := p.TextOf("resp-body"); !strings.Contains(got, "replacement") || strings.Contains(got, "first") {
		tt.Fatalf("replacement response shows stale or filtered data:\n%s", got)
	}
	for _, body := range []string{`{"broken":`, `{} {}`, "not JSON", ""} {
		resp.Body = []byte(body)
		first.showResponse(resp, nil)
		p.Advance(0)
		if _, ok := p.Bounds("resp-body-json"); ok {
			tt.Fatalf("offered a JSON tree for %q", body)
		}
		if _, ok := p.Bounds(responseJSONQueryID); ok {
			tt.Fatalf("left the old JSON view visible for %q", body)
		}
		if body != "" && !strings.Contains(p.TextOf("resp-body"), body) {
			tt.Fatalf("fallback lost the raw response %q", body)
		}
	}
	// History and misleading/missing content types use the same viewer, and
	// scalar JSON (including null) is not mistaken for an invalid document.
	for _, body := range []string{"null", "false", `"scalar"`, "[]", "{}"} {
		resp.Body, resp.Headers = []byte(body), nil
		entry := sentEntry(1, sampleRequest(tt, "List users"), resp, time.Now())
		first.showResponse(resp, &entry)
		p.Advance(0)
		p.Click("resp-body-json")
		p.Press("home", "y")
		if got := p.Clipboard(); got != body {
			tt.Fatalf("history scalar copy = %q, want %q", got, body)
		}
		app.toasts.Clear()
		p.Press("t")
	}
}

func TestResponseJSONStreamInspector(tt *testing.T) {
	app, s, updates := streamApp(tt)
	tt.Cleanup(s.Stop)
	app.toggleStream()
	drainUntil(tt, updates, func() bool { return s.streaming.Peek() == streamOpen && bodyHas(s, "hi") })
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click(messagesID)
	p.Press("end", "up", "enter", "t", "/")
	p.Type("$.text")
	p.Press("enter", "end", "y")
	if got := p.Clipboard(); got != `"hi"` {
		tt.Fatalf("message copy = %q:\n%s", got, p.ScreenText())
	}
	s.payloads[model.KindGRPC].(*grpcEditor).message.SetText(`{"text":"again"}`)
	app.send()
	drainUntil(tt, updates, func() bool { return bodyHas(s, "again") })
	p.Advance(0)
	p.Press("y")
	if got := p.Clipboard(); got != `"hi"` || !strings.Contains(p.TextOf(responseJSONQueryID), "$.text") {
		tt.Fatal("arriving messages changed the inspected JSON or its filter")
	}
	p.Press("escape", "escape") // Clear the query, then return to the log.
	if p.FocusedID() != messagesID || s.phase.Peek() != exchangeSending {
		tt.Fatal("escape should return to the log without cancelling the stream")
	}
	p.Press("end", "enter", "end", "y")
	if got := p.Clipboard(); got != `"again"` {
		tt.Fatalf("newly selected message copy = %q", got)
	}
}
