package ui

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/curl"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestCurlInURLBarImportsOnEnter(tt *testing.T) {
	app := testApp()
	s := app.current()
	command := `curl -X PUT 'https://api.test/items/7?force=1' -H 'Content-Type: application/json' --data-raw '{"a": 1}'`
	s.url.SetText(command)
	app.submitURL(command)
	if s.method.Peek() != model.MethodPut || s.url.GetText() != "https://api.test/items/7?force=1" {
		tt.Fatalf("imported %s %s", s.method.Peek(), s.url.GetText())
	}
	if got := s.query.Values(); len(got) != 1 || got[0].Name != "force" {
		tt.Fatalf("query rows = %+v", got)
	}
	if s.bodyType.Peek() != model.BodyRaw || s.body.GetText() != `{"a": 1}` || !s.dirty.Peek() {
		tt.Fatalf("body = %q (%s), dirty = %v", s.body.GetText(), s.bodyType.Peek(), s.dirty.Peek())
	}
	if s.phase.Peek() != exchangeIdle {
		tt.Fatal("importing must not send the request")
	}
}

func TestCurlLineContinuationsWaitForTheLastLine(tt *testing.T) {
	app := testApp()
	s := app.current()
	// Pasting line by line: each line ends with enter.
	s.url.SetText(`curl https://api.test/x \`)
	app.submitURL(s.url.GetText())
	if !strings.HasPrefix(s.url.GetText(), "curl ") {
		tt.Fatalf("a continued command was imported early: %q", s.url.GetText())
	}
	s.url.SetText(s.url.GetText() + `-H 'A: b'`)
	app.submitURL(s.url.GetText())
	if s.url.GetText() != "https://api.test/x" || len(s.headers.Values()) != 1 {
		tt.Fatalf("url = %q headers = %+v", s.url.GetText(), s.headers.Values())
	}
}

func TestCurlImportKeepsTheTabsIdentity(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Get user"))
	app.importCurl("curl https://other.test")
	s := app.current()
	if s.file.Peek() != "users/get-user.posting.yaml" || strings.TrimSpace(s.name.GetText()) != "Get user" {
		tt.Fatalf("file = %q name = %q", s.file.Peek(), s.name.GetText())
	}
}

func TestExportThenImportIsLossless(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	before := app.current().Snapshot()
	app.curlDialog.resolve.Set(false)
	command, err := app.exportCommand(false)
	if err != nil {
		tt.Fatal(err)
	}
	app.newTab()
	app.importCurl(command)
	after := app.current().Snapshot()
	if after.Method != before.Method || after.URL != before.URL || after.Body.Raw != before.Body.Raw ||
		after.Auth != before.Auth || len(after.Headers) != len(before.Headers) {
		tt.Fatalf("round trip changed the request\ncommand: %s\nbefore: %+v\nafter:  %+v", command, before, after)
	}

	// Resolved, variables are replaced by the environment's values.
	resolved, err := app.exportCommand(true)
	if err != nil {
		tt.Fatal(err)
	}
	req, err := curl.Parse(resolved)
	if err != nil || req.URL != "http://localhost:8000/users" || req.Auth.Token != "dev-token-123" {
		tt.Fatalf("resolved export: %v %+v\n%s", err, req, resolved)
	}
}

func TestSnapshotCurlExport(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	app.copyExport()
	t.AssertSnapshot(tt, app, snapW, snapH, "Export as curl dialog showing the resolved, multi-line command")
}

func TestSnapshotCurlHintInURLBar(tt *testing.T) {
	app := testApp()
	app.current().url.SetText("curl -X POST https://api.test")
	t.AssertSnapshot(tt, app, snapW, snapH, "A curl command typed into the URL bar, with the hint to press enter to import it")
}

// pasteInto pastes text into the focusable widget id, as the focus manager
// does when it has focus, and reports whether the paste was taken.
func pasteInto(tt *testing.T, app *App, id, text string) bool {
	tt.Helper()
	renderer := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	for _, entry := range renderer.Render(app) {
		if entry.ID != id {
			continue
		}
		handler, ok := entry.Focusable.(t.PasteHandler)
		if !ok {
			tt.Fatalf("%s (%T) doesn't take pastes", id, entry.Focusable)
		}
		return handler.HandlePaste(text)
	}
	tt.Fatalf("%s is not focusable on screen", id)
	return false
}

func TestPastingCurlIntoURLBarImportsIt(tt *testing.T) {
	app := testApp()
	s := app.current()
	pasteInto(tt, app, urlInputID, "curl -X POST https://api.test/items \\\n  -H 'Accept: text/plain' \\\n  --data-raw 'hi'\n")
	if s.method.Peek() != model.MethodPost || s.url.GetText() != "https://api.test/items" {
		tt.Fatalf("imported %s %q", s.method.Peek(), s.url.GetText())
	}
	if got := s.headers.Values(); len(got) != 1 || got[0].Value != "text/plain" || s.body.GetText() != "hi" {
		tt.Fatalf("headers = %+v body = %q", got, s.body.GetText())
	}
}

func TestPastingAURLInsertsIt(tt *testing.T) {
	app := testApp()
	s := app.current()
	s.url.SetText("https://api.test/")
	s.url.CursorEnd()
	pasteInto(tt, app, urlInputID, "users\n")
	if s.url.GetText() != "https://api.test/users" || s.method.Peek() != model.MethodGet {
		tt.Fatalf("url = %q", s.url.GetText())
	}
}
