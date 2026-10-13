package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
)

func TestURLUndoRestoresQueryToggle(tt *testing.T) {
	for _, enabled := range []bool{false, true} {
		tt.Run(map[bool]string{false: "enable", true: "disable"}[enabled], func(tt *testing.T) {
			app := testApp()
			req := model.NewRequest()
			req.URL = "https://api.test/search"
			req.Query = []model.KeyValue{{Name: "limit", Value: "10", Enabled: enabled}}
			app.openRequest(req)
			s := app.current()
			s.requestTab.Set("query")
			before := s.Snapshot()
			p := t.NewPilot(tt, app, snapW, snapH)
			t.RequestFocus(s.query.FirstInputID())
			p.Advance(0)
			p.Press("ctrl+r")
			after := s.Snapshot()
			if len(after.Query) != 1 || after.Query[0].Enabled == enabled {
				tt.Fatalf("toggle gave %+v", after.Query)
			}
			t.RequestFocus(urlInputID)
			p.Advance(0)
			p.Press("ctrl+z")
			if got := s.Snapshot(); got.URL != before.URL || !reflect.DeepEqual(got.Query, before.Query) {
				tt.Fatalf("undo URL/query = %q %+v, want %q %+v", got.URL, got.Query, before.URL, before.Query)
			}
			p.Press("ctrl+shift+z")
			if got := s.Snapshot(); got.URL != after.URL || !reflect.DeepEqual(got.Query, after.Query) {
				tt.Fatalf("redo URL/query = %q %+v, want %q %+v", got.URL, got.Query, after.URL, after.Query)
			}
		})
	}
}

func focusURLHistory(tt *testing.T, p *t.Pilot, id string) {
	tt.Helper()
	t.RequestFocus(id)
	p.Advance(0)
	if p.FocusedID() != id {
		tt.Fatalf("focus = %q, want %q", p.FocusedID(), id)
	}
}

func queryEditorRows(s *Session) []model.KeyValue {
	var rows []model.KeyValue
	for _, row := range s.query.rows.Peek() {
		rows = append(rows, model.KeyValue{Name: row.key.GetText(), Value: row.value.GetText(), Enabled: row.enabled.Peek()})
	}
	return rows
}

func TestURLUndoPreservesAllQueryRows(tt *testing.T) {
	app := testApp()
	req := model.NewRequest()
	req.URL = "https://api.test/search"
	req.Query = []model.KeyValue{
		{Name: "duplicate", Value: "one", Enabled: true},
		{Name: "hidden", Value: "kept", Enabled: false},
		{Value: "unfinished", Enabled: true},
		{Name: "duplicate", Value: "two", Enabled: true},
	}
	app.openRequest(req)
	s := app.current()
	s.requestTab.Set("query")
	p := t.NewPilot(tt, app, snapW, snapH)
	want := append(append([]model.KeyValue(nil), req.Query...), model.KeyValue{Enabled: true})
	url := "https://api.test/search?duplicate=one&duplicate=two"
	focusURLHistory(tt, p, s.query.inputID(s.query.rows.Peek()[1].id, "key"))
	p.Press("ctrl+x")
	focusURLHistory(tt, p, urlInputID)
	p.Press("ctrl+z")
	if got := queryEditorRows(s); !reflect.DeepEqual(got, want) || s.url.GetText() != url {
		tt.Fatalf("undo deleted row = %q %+v, want %q %+v", s.url.GetText(), got, url, want)
	}
	p.Press("ctrl+shift+z")
	after := []model.KeyValue{req.Query[0], req.Query[2], req.Query[3], {Enabled: true}}
	if got := queryEditorRows(s); !reflect.DeepEqual(got, after) || s.url.GetText() != url {
		tt.Fatalf("redo deleted row = %q %+v, want %q %+v", s.url.GetText(), got, url, after)
	}
}

func TestURLUndoRecordsQueryOnlyEdits(tt *testing.T) {
	for _, row := range []model.KeyValue{
		{Name: "disabled", Value: "first", Enabled: false},
		{Value: "unfinished", Enabled: true},
	} {
		tt.Run(row.Value, func(tt *testing.T) {
			app := testApp()
			req := model.NewRequest()
			req.URL = "https://api.test/search"
			req.Query = []model.KeyValue{row}
			app.openRequest(req)
			s := app.current()
			s.requestTab.Set("query")
			p := t.NewPilot(tt, app, snapW, snapH)
			focusURLHistory(tt, p, s.query.inputID(s.query.rows.Peek()[0].id, "value"))
			p.Press("ctrl+a")
			p.Paste("changed")
			if s.url.GetText() != req.URL {
				tt.Fatalf("row-only edit changed URL to %q", s.url.GetText())
			}
			focusURLHistory(tt, p, urlInputID)
			p.Press("ctrl+z")
			want := []model.KeyValue{row, {Enabled: true}}
			if got := queryEditorRows(s); !reflect.DeepEqual(got, want) {
				tt.Fatalf("undo query-only edit = %+v, want %+v", got, want)
			}
			p.Press("ctrl+shift+z")
			want[0].Value = "changed"
			if got := queryEditorRows(s); !reflect.DeepEqual(got, want) {
				tt.Fatalf("redo query-only edit = %+v, want %+v", got, want)
			}
			p.Press("ctrl+z")
			focusURLHistory(tt, p, s.query.inputID(s.query.rows.Peek()[0].id, "value"))
			p.Press("ctrl+a")
			p.Paste("branch")
			focusURLHistory(tt, p, urlInputID)
			p.Press("ctrl+shift+z")
			want[0].Value = "branch"
			if got := queryEditorRows(s); !reflect.DeepEqual(got, want) {
				tt.Fatalf("query-only edit retained an abandoned redo branch: %+v", got)
			}
		})
	}
}

func TestURLUndoTypingAndDeletionGroups(tt *testing.T) {
	app := testApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("one two")
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "one " {
		tt.Fatalf("word undo = %q, want one plus a space", got)
	}
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "" {
		tt.Fatalf("typing undo = %q, want empty", got)
	}
	p.Press("ctrl+shift+z", "ctrl+shift+z")
	p.Press("backspace", "backspace")
	if got := s.url.GetText(); got != "one t" {
		tt.Fatalf("backspace = %q", got)
	}
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "one two" {
		tt.Fatalf("backspace undo = %q", got)
	}
	p.Press("home", "delete", "delete")
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "one two" || s.url.CursorIndex.Peek() != 0 {
		tt.Fatalf("delete undo = %q at %d", got, s.url.CursorIndex.Peek())
	}
	p.Type("!")
	p.Press("ctrl+shift+z")
	if got := s.url.GetText(); got != "!one two" {
		tt.Fatalf("new edit retained abandoned redo branch: %q", got)
	}
}

func TestURLUndoTypingMovementAndPasteBoundaries(tt *testing.T) {
	app := testApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("ab")
	p.Press("left", "right")
	p.Type("cd")
	p.Paste("ef")
	for _, want := range []string{"abcd", "ab", ""} {
		p.Press("ctrl+z")
		if got := s.url.GetText(); got != want {
			tt.Fatalf("undo = %q, want %q", got, want)
		}
	}
}

func TestURLUndoCompletionAndTyping(tt *testing.T) {
	app := urlCompletionApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("http://local")
	p.Press("tab")
	p.Type("/us")
	p.Press("tab")
	if got := s.url.GetText(); got != "http://localhost:3000/users" {
		tt.Fatalf("completion = %q", got)
	}
	for _, want := range []string{"http://localhost:3000/us", "http://localhost:3000", "http://local", ""} {
		p.Press("ctrl+z")
		if got := s.url.GetText(); got != want {
			tt.Fatalf("undo completion = %q, want %q", got, want)
		}
	}
	for _, want := range []string{"http://local", "http://localhost:3000", "http://localhost:3000/us", "http://localhost:3000/users"} {
		p.Press("ctrl+shift+z")
		if got := s.url.GetText(); got != want {
			tt.Fatalf("redo completion = %q, want %q", got, want)
		}
	}
}

func TestURLUndoRestoresUnicodeSelection(tt *testing.T) {
	app := testApp()
	req := model.NewRequest()
	req.URL = "https://api.test/cafe\u0301/猫"
	app.openRequest(req)
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Press("end", "shift+left", "shift+left", "shift+left")
	cursor, anchor := s.url.CursorIndex.Peek(), s.url.SelectionAnchor.Peek()
	p.Type("xyz")
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != req.URL || s.url.CursorIndex.Peek() != cursor || s.url.SelectionAnchor.Peek() != anchor {
		tt.Fatalf("undo selection = %q at %d/%d, want %q at %d/%d", got, s.url.CursorIndex.Peek(), s.url.SelectionAnchor.Peek(), req.URL, cursor, anchor)
	}
	p.Press("ctrl+shift+z")
	if got := s.url.GetText(); got != "https://api.test/cafxyz" || s.url.SelectionAnchor.Peek() != -1 {
		tt.Fatalf("redo selection = %q at %d/%d", got, s.url.CursorIndex.Peek(), s.url.SelectionAnchor.Peek())
	}
}

func TestURLUndoInterleavesQueryAndURL(tt *testing.T) {
	app := testApp()
	req := model.NewRequest()
	req.URL = "https://api.test/search"
	req.Query = []model.KeyValue{{Name: "same", Value: "one", Enabled: false}, {Name: "same", Value: "two", Enabled: true}}
	app.openRequest(req)
	s := app.current()
	s.requestTab.Set("query")
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, s.query.FirstInputID())
	p.Press("ctrl+r")
	focusURLHistory(tt, p, urlInputID)
	p.Press("end")
	p.Type("#fragment")
	p.Press("ctrl+z")
	want := []model.KeyValue{{Name: "same", Value: "one", Enabled: true}, {Name: "same", Value: "two", Enabled: true}, {Enabled: true}}
	if got := queryEditorRows(s); !reflect.DeepEqual(got, want) || s.url.GetText() != "https://api.test/search?same=one&same=two" {
		tt.Fatalf("undo URL edit = %q %+v", s.url.GetText(), got)
	}
	p.Press("ctrl+z")
	want[0].Enabled = false
	if got := queryEditorRows(s); !reflect.DeepEqual(got, want) || s.url.GetText() != "https://api.test/search?same=two" {
		tt.Fatalf("undo query edit = %q %+v", s.url.GetText(), got)
	}
	p.Press("ctrl+shift+z", "ctrl+shift+z")
	if got := s.url.GetText(); got != "https://api.test/search?same=one&same=two#fragment" {
		tt.Fatalf("redo interleaved edits = %q", got)
	}
}

func TestURLUndoCannotCrossImportOrLoad(tt *testing.T) {
	app := testApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("https://old.test/?a=b")
	p.Paste("curl https://imported.test/?x=y")
	p.Press("ctrl+z", "ctrl+shift+z")
	if got := s.url.GetText(); got != "https://imported.test/?x=y" {
		tt.Fatalf("undo crossed curl import: %q", got)
	}
	p.Press("end")
	p.Type("#edit")
	req := model.NewRequest()
	req.URL = "https://loaded.test/"
	s.Load(req)
	p.Advance(0)
	p.Press("ctrl+z", "ctrl+shift+z")
	if got := s.url.GetText(); got != req.URL || len(s.query.Values()) != 0 {
		tt.Fatalf("undo crossed load: %q %+v", got, s.query.Values())
	}
}

func TestURLUndoHistoryIsBounded(tt *testing.T) {
	app := testApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	for i := range maxURLUndoSteps + 5 {
		s.editURL(urlSingleEdit, func() {
			s.url.ReplaceText(fmt.Sprint(i), 0)
			s.urlEdited()
		})
	}
	for range maxURLUndoSteps + 5 {
		p.Press("ctrl+z")
	}
	if got := s.url.GetText(); got != "4" {
		tt.Fatalf("history retained the wrong number of edits: undo stopped at %q, want 4", got)
	}

	// Query-only edits also count against the byte budget.
	req := model.NewRequest()
	req.URL = "https://api.test/"
	req.Query = []model.KeyValue{{Name: "disabled", Enabled: false}}
	s.Load(req)
	for _, value := range []string{"small", strings.Repeat("a", maxURLUndoBytes/2), "latest"} {
		s.query.rows.Peek()[0].value.SetText(value)
		s.queryEdited()
	}
	p.Advance(0)
	p.Press("ctrl+z", "ctrl+z")
	if got := s.query.Values()[0].Value; got != strings.Repeat("a", maxURLUndoBytes/2) {
		tt.Fatalf("history retained snapshots beyond its byte budget: got %d bytes", len(got))
	}
	p.Press("ctrl+shift+z")
	if got := s.query.Values()[0].Value; got != "latest" {
		tt.Fatalf("newest large edit was not retained: %q", got)
	}
}

func TestURLUndoTypingPauseStartsANewGroup(tt *testing.T) {
	app := testApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("abc")
	time.Sleep(time.Second + 10*time.Millisecond)
	p.Type("d")
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "abc" {
		tt.Fatalf("typing across a pause was grouped: %q", got)
	}
}

func TestURLUndoMouseCompletion(tt *testing.T) {
	app := urlCompletionApp()
	s := app.current()
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	p.Type("http://localhost:3000/teams/m")
	p.Click("-list")
	p.Click("-list")
	if got := s.url.GetText(); got != "http://localhost:3000/teams/members" {
		tt.Fatalf("mouse completion = %q", got)
	}
	focusURLHistory(tt, p, urlInputID)
	p.Press("ctrl+z")
	if got := s.url.GetText(); got != "http://localhost:3000/teams/m" {
		tt.Fatalf("undo mouse completion = %q", got)
	}
	p.Press("ctrl+shift+z")
	if got := s.url.GetText(); got != "http://localhost:3000/teams/members" {
		tt.Fatalf("redo mouse completion = %q", got)
	}
}

func TestURLUndoDropsAnOversizedStep(tt *testing.T) {
	app := testApp()
	s := app.current()
	req := model.NewRequest()
	req.URL = "https://api.test/"
	req.Query = []model.KeyValue{{Name: "disabled", Enabled: false}}
	s.Load(req)
	p := t.NewPilot(tt, app, snapW, snapH)
	focusURLHistory(tt, p, urlInputID)
	large := strings.Repeat("x", maxURLUndoBytes+1)
	s.query.rows.Peek()[0].value.SetText(large)
	s.queryEdited()
	p.Press("ctrl+z")
	if got := s.query.Values()[0].Value; got != large {
		tt.Fatalf("oversized undo step was retained: got %d bytes", len(got))
	}
	s.query.rows.Peek()[0].value.SetText("small")
	s.queryEdited()
	p.Press("ctrl+z", "ctrl+shift+z")
	if got := s.query.Values()[0].Value; got != "small" {
		tt.Fatalf("history retained the oversized previous state: got %d bytes", len(got))
	}
}
