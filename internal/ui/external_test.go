package ui

import (
	"os"
	"os/exec"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

// fakeProgram stands in for the editor or pager: it records the command and
// the file it was given, then writes replacement into the file.
type fakeProgram struct {
	args        []string
	given       string
	replacement string
}

func (p *fakeProgram) run(cmd *exec.Cmd) error {
	p.args = cmd.Args
	file := cmd.Args[len(cmd.Args)-1]
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	p.given = string(data)
	if p.replacement == "" {
		return nil
	}
	return os.WriteFile(file, []byte(p.replacement), 0o600)
}

func TestEditorEditsTheRequestBody(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	s := app.current()
	s.requestTab.Set("body")
	s.dirty.Set(false)
	editor := &fakeProgram{replacement: `{"name": "edited"}`}
	app.runExternal = editor.run
	app.settings.Editor = "code --wait"

	before := s.body.GetText()
	pressOn(tt, app, "req-body-text", "f4")
	if len(editor.args) != 3 || editor.args[0] != "code" || editor.args[1] != "--wait" {
		tt.Fatalf("ran %q", editor.args)
	}
	if editor.given != before {
		tt.Fatalf("the editor was given %q, want %q", editor.given, before)
	}
	if s.body.GetText() != `{"name": "edited"}` || !s.dirty.Peek() {
		tt.Fatalf("body = %q, dirty = %v", s.body.GetText(), s.dirty.Peek())
	}
	if _, err := os.Stat(editor.args[2]); !os.IsNotExist(err) {
		tt.Fatal("the temporary file should be removed")
	}
}

func TestPagerShowsTheResponseBody(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	pager := &fakeProgram{replacement: "ignored"}
	app.runExternal = pager.run
	app.settings.Pager = "less"
	app.settings.PagerJSON = "jless"

	body := s.responseBody.GetText()
	pressOn(tt, app, "resp-body", "f3")
	if len(pager.args) != 2 || pager.args[0] != "jless" {
		tt.Fatalf("a JSON response should use the JSON pager; ran %q", pager.args)
	}
	if pager.given != body {
		tt.Fatalf("the pager was given %q", pager.given)
	}

	// The response is read-only, so the editor only shows it.
	app.settings.Editor = "vim"
	pressOn(tt, app, "resp-body", "f4")
	if pager.args[0] != "vim" || s.responseBody.GetText() != body {
		tt.Fatalf("ran %q; body = %q", pager.args, s.responseBody.GetText())
	}
}

func TestExternalProgramNeedsConfiguring(tt *testing.T) {
	app := testApp()
	s := app.current()
	s.bodyType.Set(model.BodyRaw)
	s.requestTab.Set("body")
	ran := false
	app.runExternal = func(*exec.Cmd) error { ran = true; return nil }
	app.settings.Editor = ""
	pressOn(tt, app, "req-body-text", "f4")
	if ran || app.toast.Peek().message == "" {
		tt.Fatalf("ran = %v, toast = %q", ran, app.toast.Peek().message)
	}
}
