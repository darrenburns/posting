package ui

import (
	"fmt"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// screen drives an app through the renderer the way the app loop does, so
// tests can move focus and check what ends up on screen.
type screen struct {
	app      *App
	renderer *t.Renderer
	focus    *t.FocusManager
	focused  t.AnySignal[t.Focusable]
}

func newScreen(app *App, width, height int) *screen {
	s := &screen{app: app, focus: t.NewFocusManager(), focused: t.NewAnySignal[t.Focusable](nil)}
	s.renderer = t.NewRenderer(uv.NewBuffer(width, height), width, height, s.focus, s.focused, t.NewAnySignal[t.Widget](nil))
	s.render()
	// The first frame measures the screen and dispatches the switch to
	// compact spacing. A running app renders again after dispatched work;
	// outside one, Dispatch runs immediately, mid-frame, so force that frame.
	s.renderer.Resize(width, height)
	s.render()
	return s
}

func (s *screen) render() {
	s.focus.SetFocusables(s.renderer.Update(s.app))
}

func (s *screen) focusID(tt *testing.T, id string) {
	tt.Helper()
	s.focus.FocusByID(id)
	if s.focus.FocusedID() != id {
		tt.Fatalf("couldn't focus %s", id)
	}
	s.focused.Set(s.focus.Focused())
	s.render()
}

func (s *screen) visible(id string) bool {
	entry := s.renderer.WidgetByID(id)
	return entry != nil && !entry.Visible.IsEmpty()
}

func TestShortTerminalIsCompact(tt *testing.T) {
	// The URL bar sits under the request tabs, with a blank row between them
	// unless the terminal is short.
	for _, tc := range []struct {
		height, urlRow int
	}{{snapH, 3}, {20, 2}} {
		s := newScreen(testApp(), snapW, tc.height)
		if got := s.renderer.WidgetByID(urlInputID).Visible.Y; got != tc.urlRow {
			tt.Errorf("%d rows: URL bar on row %d, want %d", tc.height, got, tc.urlRow)
		}
	}
}

func TestFocusScrollsRequestOptionsIntoView(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	app.current().requestTab.Set("options")
	s := newScreen(app, snapW, 20)
	if s.visible("req-opt-timeout") {
		tt.Fatal("the timeout field fits on a 20-row screen, so this test proves nothing")
	}
	s.focusID(tt, "req-opt-timeout")
	if !s.visible("req-opt-timeout") {
		tt.Error("focusing the timeout field didn't scroll it into view")
	}
	s.focusID(tt, "req-opt-follow")
	if !s.visible("req-opt-follow") {
		tt.Error("focusing the first checkbox didn't scroll back up to it")
	}
}

func TestFocusScrollsKeyValueRowIntoView(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	editor := app.current().headers
	var headers []model.KeyValue
	for i := 1; i <= 30; i++ {
		headers = append(headers, model.KeyValue{Name: fmt.Sprintf("X-Header-%d", i), Value: "v", Enabled: true})
	}
	editor.Load(headers)
	s := newScreen(app, snapW, snapH)
	// Tab, a jump or a click can focus a row without the arrow keys.
	far := editor.inputID(editor.rowAt(25).id, "value")
	if s.visible(far) {
		tt.Fatal("row 25 fits on screen, so this test proves nothing")
	}
	s.focusID(tt, far)
	if !s.visible(far) {
		tt.Error("focusing a header row below the panel didn't scroll it into view")
	}
	first := editor.FirstInputID()
	s.focusID(tt, first)
	if !s.visible(first) {
		tt.Error("focusing the first header row didn't scroll back up to it")
	}
}

func TestFocusScrollsSaveButtonsIntoView(tt *testing.T) {
	app := testApp()
	app.save.prefill(app.current().Snapshot(), "")
	app.overlay.Set("save")
	s := newScreen(app, snapW, 14)
	if s.visible("save-submit") {
		tt.Fatal("the save button fits on a 14-row screen, so this test proves nothing")
	}
	s.focusID(tt, "save-submit")
	if !s.visible("save-submit") {
		tt.Error("focusing the save button didn't scroll it into view")
	}
}
