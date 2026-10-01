package ui

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/darrenburns/posting/internal/model"
	t "github.com/darrenburns/terma"
	"strings"
	"testing"
)

func TestResponseBodyVisualPaging(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", strings.Repeat("hello world\n", 100))
	press(tt, app, "v", "l", "pgdown")
	copied, selected := s.bodyCopyText()
	tt.Logf("visual=%v anchor=%d cursor=%d copied=%d bytes selected=%v", s.responseVisual.Peek(), s.responseBody.SelectionAnchor.Peek(), s.responseBody.CursorIndex.Peek(), len(copied), selected)
	if s.responseBody.SelectionAnchor.Peek() != 0 || !selected {
		tt.Fatal("PageDown discarded the visual selection")
	}
}

func TestResponseBodyBackwardInclusive(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world")
	press(tt, app, "w", "v", "h")
	got, _ := s.bodyCopyText()
	if got != " w" {
		tt.Fatalf("backward visual copy=%q, want both anchor and cursor characters %q", got, " w")
	}
}

func TestResponseBodyVisualMouseSelection(tt *testing.T) {
	for _, clicks := range []int{2, 3} {
		tt.Run(string(rune('0'+clicks)), func(tt *testing.T) {
			app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
			press(tt, app, "v")
			r := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
			r.Render(app)
			entry := r.WidgetByID("resp-body")
			area := entry.EventWidget.(t.TextArea)
			area.OnMouseDown(t.MouseEvent{LocalX: 1, LocalY: 0, Button: uv.MouseLeft, ClickCount: clicks})
			expected := s.responseBody.GetSelectedText()
			got, _ := s.bodyCopyText()
			if got != expected {
				tt.Fatalf("%d clicks selected=%q, copied=%q, visual=%v", clicks, expected, got, s.responseVisual.Peek())
			}
		})
	}
}

func TestResponseBodyTopResetsColumn(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "abcdefgh\nabcdefgh\nabcdefgh")
	press(tt, app, "l", "l", "l", "j", "g", "j")
	if got := cursorAt(s); got != "2:1" {
		tt.Fatalf("after column 4, g,j lands %s, want 2:1", got)
	}
}

func TestResponseBodySmartHomeResetsColumn(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "  abcdefgh\n  abcdefgh\n  abcdefgh")
	press(tt, app, "$", "0", "j")
	if got := cursorAt(s); got != "2:3" {
		tt.Fatalf("after $,0,j lands %s, want 2:3", got)
	}
}

func TestResponseBodyLegacyPagingKeys(tt *testing.T) {
	app, _ := bodyApp(tt, "text/plain", strings.Repeat("hello world\n", 100))
	keys := bodyKeys(tt, app)
	for _, key := range []string{"ctrl+b", "ctrl+f", "ctrl+d", "ctrl+u"} {
		if !keys[key] {
			tt.Errorf("missing legacy paging binding %s", key)
		}
	}
}

func TestResponseBodyBoundaryControls(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "é👩‍💻漢字\nsecond")
	press(tt, app, "v", "l")
	if got, _ := s.bodyCopyText(); got != "é👩‍💻" {
		tt.Fatalf("unicode copy=%q", got)
	}
	s.setBodyVisual(false)
	s.responseBody.SetText("")
	s.setBodyVisual(true)
	for _, motion := range bodyMotions {
		s.moveBodyCursor(motion.move, false)()
	}
	if got, _ := s.bodyCopyText(); got != "" {
		tt.Fatalf("empty copy=%q", got)
	}
}

func TestResponseBodyMouseCopyOutsideVisual(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
	r := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	r.Render(app)
	area := r.WidgetByID("resp-body").EventWidget.(t.TextArea)
	area.OnMouseDown(t.MouseEvent{LocalX: 1, LocalY: 0, Button: uv.MouseLeft, ClickCount: 2})
	if got, _ := s.bodyCopyText(); got != "hello" {
		tt.Fatalf("normal double click copy=%q", got)
	}
}

func TestResponseBodyFocusAndTabIsolation(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
	s.bodyType.Set(model.BodyRaw)
	s.body.SetText("request body")
	s.requestTab.Set("body")
	fm := t.NewFocusManager()
	r := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, fm, t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	render := func() { fm.SetFocusables(r.Render(app)) }
	focus := func(id string) {
		render()
		fm.FocusByID(id)
		if fm.FocusedID() != id {
			tt.Fatalf("focus=%s, want %s", fm.FocusedID(), id)
		}
	}
	key := func(key string) {
		provider := fm.Focused().(t.KeybindProvider)
		for _, kb := range provider.Keybinds() {
			if kb.Key == key {
				kb.Action()
				render()
				return
			}
		}
		tt.Fatalf("focused %s lacks %s", fm.FocusedID(), key)
	}
	unchanged := func() {
		if got, selected := s.bodyCopyText(); got != "he" || !selected || !s.responseVisual.Peek() {
			tt.Fatalf("response changed: copy=%q selected=%v visual=%v", got, selected, s.responseVisual.Peek())
		}
	}
	focus("resp-body")
	key("v")
	key("l")
	unchanged()
	for _, id := range []string{urlInputID, "req-body-text"} {
		focus(id)
		for _, kb := range fm.Focused().(t.KeybindProvider).Keybinds() {
			if kb.Key == "y" || kb.Key == "c" {
				tt.Fatalf("response copy binding leaked to %s", id)
			}
		}
		before := ""
		if id == urlInputID {
			before = s.url.GetText()
		} else {
			before = s.body.GetText()
		}
		key("end")
		key("backspace")
		after := ""
		if id == urlInputID {
			after = s.url.GetText()
		} else {
			after = s.body.GetText()
		}
		if before == after {
			tt.Fatalf("backspace did not edit %s", id)
		}
		unchanged()
		focus("resp-body")
		unchanged()
		tt.Logf("focus %s, edit, return preserves response selection and has no y/c response binding", id)
	}
	focus(responseTabsID)
	key("right")
	if s.responseTab.Peek() != "headers" {
		tt.Fatal("right did not open Headers")
	}
	unchanged()
	key("left")
	focus("resp-body")
	unchanged()
	tt.Log("response Body -> Headers -> Body preserves visual selection")
	s2 := app.openSession(sampleRequest(tt, "Create user"))
	resp := fixedResponse()
	resp.Headers = []model.Header{{Name: "Content-Type", Value: "text/plain"}}
	resp.Body = []byte("different response")
	s2.showResponse(resp, nil)
	s2.phase.Set(exchangeDone)
	render()
	focus("resp-body")
	if s2.responseVisual.Peek() || s2.responseBody.HasSelection() {
		tt.Fatal("selection leaked to second session")
	}
	key("v")
	key("l")
	key("l")
	if got, _ := s2.bodyCopyText(); got != "dif" {
		tt.Fatalf("second session selection=%q", got)
	}
	app.cycleSession(-1)
	render()
	if app.current() != s {
		tt.Fatal("did not return to first session")
	}
	focus("resp-body")
	unchanged()
	app.cycleSession(1)
	render()
	if app.current() != s2 {
		tt.Fatal("did not return to second session")
	}
	if got, _ := s2.bodyCopyText(); got != "dif" {
		tt.Fatalf("second selection changed=%q", got)
	}
	tt.Log("session switching preserves independent he/dif visual selections")
	app.cycleSession(-1)
	render()
	focus("resp-body")
	area := r.WidgetByID("resp-body").EventWidget.(t.TextArea)
	area.OnMouseDown(t.MouseEvent{LocalX: 1, LocalY: 0, Button: uv.MouseLeft, ClickCount: 2})
	actual, _ := s.bodyCopyText()
	if actual != "hello" {
		tt.Fatalf("mouse selection copy incorrect, got %q", actual)
	}
	tt.Logf("after all focus/tab changes double-click selects %q but copies %q", s.responseBody.GetSelectedText(), actual)
	if got, _ := s2.bodyCopyText(); got != "dif" {
		tt.Fatalf("mouse affected other session=%q", got)
	}
}
