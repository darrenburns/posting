package ui

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/config"
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
	// One frame is enough: work the frame dispatches settles before it returns.
	s.renderer = t.NewRenderer(uv.NewBuffer(width, height), width, height, s.focus, s.focused, t.NewAnySignal[t.Widget](nil))
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

func TestStandardSpacingSurvivesResizeAndLegacyConfig(tt *testing.T) {
	for _, spacing := range []string{"standard", "compact"} {
		tt.Run(spacing, func(tt *testing.T) {
			app := settingsApp(tt, func(s *config.Settings) { s.Spacing = spacing })
			p := t.NewPilot(tt, app, snapW, snapH)
			for _, height := range []int{snapH, 28, 20, snapH} {
				p.Resize(snapW, height)
				for _, id := range []string{urlInputID, treeSearchID} {
					bounds, ok := p.Bounds(id)
					if !ok || bounds.Y != 3 {
						tt.Errorf("%d rows: %s on row %d, want 3 with a blank row above it", height, id, bounds.Y)
					}
				}
			}
		})
	}
}

func TestFocusScrollsRequestOptionsIntoView(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	app.current().requestTab.Set("options")
	s := newScreen(app, snapW, 19)
	if s.visible("req-opt-timeout") {
		tt.Fatal("the timeout field fits on a 19-row screen, so this test proves nothing")
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

func TestSidebarScrollbarTrackMatchesFractionalThumb(tt *testing.T) {
	for _, tab := range []string{"requests", "history"} {
		tt.Run(tab, func(tt *testing.T) {
			app := testApp()
			collection := &model.Collection{Name: "Requests"}
			var history []model.HistoryEntry
			for i := 0; i < 61; i++ {
				req := model.NewRequest()
				req.Name = fmt.Sprintf("Request %02d", i)
				collection.Requests = append(collection.Requests, req)
				history = append(history, model.HistoryEntry{ID: int64(i + 1), Request: req})
			}
			app.tree.Nodes.Set(buildTree(collection))
			app.history.Set(history)
			app.historyList.SetItems(history)
			var view t.Widget = collectionView{app: app}
			state := app.treeScroll
			if tab == "history" {
				view, state = historyView{app: app}, app.historyScroll
			}
			theme, _ := t.GetTheme("galaxy")
			background, _ := app.sidebarColors(theme)
			root := t.Column{
				Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: background},
				Children: []t.Widget{view},
			}
			const width, height = 40, 20
			t.RenderToBuffer(root, width, height)
			state.SetOffset(7)
			buf := t.SerializeBuffer(t.RenderToBuffer(root, width, height), width, height)
			track := buf.Cells[width-1].Bg
			thumb := strings.ToLower(theme.ScrollbarThumb.Hex())
			if track == thumb || track == strings.ToLower(theme.ScrollbarTrack.Hex()) {
				tt.Fatal("track must be softened without changing the thumb")
			}
			partial := 0
			for y := 0; y < height; y++ {
				cell := buf.Cells[y*width+width-1]
				if cell.Content != "" && strings.Contains("▁▂▃▄▅▆▇", cell.Content) {
					partial++
					if !((cell.Fg == track && cell.Bg == thumb) || (cell.Fg == thumb && cell.Bg == track)) {
						tt.Errorf("fractional thumb at row %d = %+v, want track %s and thumb %s", y, cell, track, thumb)
					}
				} else if cell.Bg != track && cell.Bg != thumb {
					tt.Errorf("scrollbar at row %d = %+v, want track %s or thumb %s", y, cell, track, thumb)
				}
			}
			if partial != 2 {
				tt.Fatalf("need both fractional thumb ends, got %d", partial)
			}
		})
	}
}
