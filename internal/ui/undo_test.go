package ui

import (
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestUndoAfterQueryEditRestoresTheTypedURL(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Get user"))
	s := app.current()
	s.url.CursorEnd()
	pasteInto(tt, app, urlInputID, "?q=typed")
	typed := s.url.GetText()

	rows := s.query.rows.Peek()
	rows[0].value.CursorEnd()
	rows[0].value.Insert("2")
	s.queryEdited()
	if s.url.GetText() == typed {
		tt.Fatalf("editing the query table didn't rebuild the URL %q", typed)
	}

	pressOn(tt, app, urlInputID, "ctrl+z")
	if got := s.url.GetText(); got != typed {
		tt.Fatalf("after undo url = %q, want %q", got, typed)
	}
	if got := s.query.Values(); len(got) != 1 || got[0].Value != "typed" {
		tt.Fatalf("undo left the query rows out of step with the URL: %+v", got)
	}
}

func TestUndoRejoinsACurlLineContinuation(tt *testing.T) {
	app := testApp()
	s := app.current()
	// Typed line by line, without bracketed paste.
	s.url.SetText(`curl https://api.test/x \`)
	app.submitURL(s.url.GetText())
	if got := s.url.GetText(); strings.HasSuffix(got, `\`) {
		tt.Fatalf("url = %q, want the continuation joined", got)
	}
	pressOn(tt, app, urlInputID, "ctrl+z")
	if got := s.url.GetText(); got != `curl https://api.test/x \` {
		tt.Fatalf("after undo url = %q", got)
	}
}

// A tab given a different request must not undo back into the old one's
// text.
func TestUndoStopsAtALoadedRequest(tt *testing.T) {
	for _, tc := range []struct {
		name string
		load func(app *App, s *Session)
	}{
		{"curl import", func(app *App, s *Session) {
			pasteInto(tt, app, urlInputID, "curl https://imported.test/items")
		}},
		{"reload from disk", func(app *App, s *Session) {
			s.dirty.Set(false)
			root := model.SampleCollection()
			root.Walk(func(folder *model.Collection, r model.Request) {
				for i := range folder.Requests {
					if folder.Requests[i].File == s.file.Peek() {
						folder.Requests[i].URL = "https://reloaded.test/users"
					}
				}
			})
			app.replaceCollection(root)
		}},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := testApp()
			app.openRequest(sampleRequest(tt, "Get user"))
			s := app.current()
			s.url.CursorEnd()
			pasteInto(tt, app, urlInputID, "/edited")
			tc.load(app, s)
			loaded := s.url.GetText()
			if loaded == "" || loaded == sampleRequest(tt, "Get user").URL+"/edited" {
				tt.Fatalf("the request wasn't replaced: url = %q", loaded)
			}

			pressOn(tt, app, urlInputID, "ctrl+z")
			if got := s.url.GetText(); got != loaded {
				tt.Fatalf("undo reached the previous request: url = %q, want %q", got, loaded)
			}
		})
	}
}

// Saved query rows are written into a URL without a query when the request
// opens. That is part of opening it, not a step to undo.
func TestUndoKeepsSavedQueryRowsOfAnOpenedRequest(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	opened := s.url.GetText()
	pressOn(tt, app, urlInputID, "ctrl+z")
	if got := s.url.GetText(); got != opened {
		tt.Fatalf("after undo url = %q, want %q", got, opened)
	}
}

func TestUndoInTheBodyEditor(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	s := app.current()
	s.requestTab.Set("body")
	original := s.body.GetText()

	pasteInto(tt, app, "req-body-text", "// typed\n")
	edited := s.body.GetText()
	if edited == original {
		tt.Fatal("the paste didn't reach the body")
	}
	pressOn(tt, app, "req-body-text", "ctrl+z")
	if got := s.body.GetText(); got != original {
		tt.Fatalf("after undo body = %q, want %q", got, original)
	}
	pressOn(tt, app, "req-body-text", "ctrl+y")
	if got := s.body.GetText(); got != edited {
		tt.Fatalf("after redo body = %q", got)
	}
}

func TestUndoRevertsTheExternalEditor(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	s := app.current()
	s.requestTab.Set("body")
	original := s.body.GetText()
	app.runExternal = (&fakeProgram{replacement: `{"name": "edited"}`}).run
	app.settings.Editor = "vim"

	pressOn(tt, app, "req-body-text", "f4")
	if got := s.body.GetText(); got != `{"name": "edited"}` {
		tt.Fatalf("body = %q", got)
	}
	pressOn(tt, app, "req-body-text", "ctrl+z")
	if got := s.body.GetText(); got != original {
		tt.Fatalf("after undo body = %q, want %q", got, original)
	}
}

func TestUndoRestoresAGRPCMessageReplacedByItsTemplate(tt *testing.T) {
	sc, e, _, updates := grpcScreen(tt, grpcRequest())
	sc.focusID(tt, grpcMethodID)
	(<-updates)()
	methods := e.catalog.Peek().schema.Methods
	e.pick(methods[1])
	e.message.SetText(`[{"text": "mine"}]`)
	if !e.insertTemplate() || e.message.GetText() != methods[1].Template {
		tt.Fatalf("inserting the template gave %q", e.message.GetText())
	}
	sc.focusID(tt, grpcMessageID)
	sc.pressKey(tt, "ctrl+z")
	if got := e.message.GetText(); got != `[{"text": "mine"}]` {
		tt.Fatalf("after undo message = %q", got)
	}
}
