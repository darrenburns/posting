package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
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
	command, err := app.curlCommand(false)
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
	resolved, err := app.curlCommand(true)
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
	app.copyAsCurl()
	t.AssertSnapshot(tt, app, snapW, snapH, "Export as curl dialog showing the resolved, multi-line command")
}

func TestSnapshotCurlHintInURLBar(tt *testing.T) {
	app := testApp()
	app.current().url.SetText("curl -X POST https://api.test")
	t.AssertSnapshot(tt, app, snapW, snapH, "A curl command typed into the URL bar, with the hint to press enter to import it")
}
