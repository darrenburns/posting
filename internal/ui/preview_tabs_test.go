package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
)

// tabTitles lists the open tabs, with the preview tab's title in slashes.
func tabTitles(app *App) string {
	var titles []string
	for _, s := range app.sessions.Peek() {
		title := s.title.Peek()
		if s.preview.Peek() {
			title = "/" + title + "/"
		}
		titles = append(titles, title)
	}
	return strings.Join(titles, ", ")
}

func assertTabs(tt *testing.T, app *App, want string) {
	tt.Helper()
	if got := tabTitles(app); got != want {
		tt.Fatalf("tabs = %s, want %s", got, want)
	}
}

func assertActive(tt *testing.T, app *App, title string) {
	tt.Helper()
	if got := app.current().title.Peek(); got != title {
		tt.Fatalf("active tab = %q, want %q", got, title)
	}
}

func TestOpeningRequestsReusesThePreviewTab(tt *testing.T) {
	app := testApp()
	// The untouched blank tab at startup becomes the preview.
	app.openRequest(sampleRequest(tt, "List users"))
	assertTabs(tt, app, "/List users/")
	app.openRequest(sampleRequest(tt, "Get user"))
	assertTabs(tt, app, "/Get user/")
	assertActive(tt, app, "Get user")
}

func TestPreviewReplacesInPlaceBesidePermanentTabs(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	app.keepSession(app.active.Peek())
	app.newTab()
	app.current().name.SetText("Scratch")
	app.current().touch()
	app.showSession(app.sessions.Peek()[0].id)

	app.openRequest(sampleRequest(tt, "Get user"))
	assertTabs(tt, app, "List users, Scratch, /Get user/")
	// Replacing the preview keeps its place in the strip, even from another tab.
	app.showSession(app.sessions.Peek()[0].id)
	app.openRequest(sampleRequest(tt, "Login"))
	assertTabs(tt, app, "List users, Scratch, /Login/")
	assertActive(tt, app, "Login")
}

func TestReplacedPreviewStartsFresh(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	s.requestTab.Set("auth")
	app.openRequest(sampleRequest(tt, "Get user"))
	next := app.current()
	if next == s {
		tt.Fatal("the preview was reloaded in place rather than replaced")
	}
	if next.response.Peek() != nil || next.phase.Peek() != exchangeIdle || next.requestTab.Peek() != "headers" {
		tt.Error("the old preview's response or view state carried over to the new request")
	}
}

func TestOpeningAnOpenRequestShowsItsTab(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	app.keepSession(app.active.Peek())
	app.openRequest(sampleRequest(tt, "Get user"))
	app.openRequest(sampleRequest(tt, "List users"))
	assertTabs(tt, app, "List users, /Get user/")
	assertActive(tt, app, "List users")
	// The preview is its own tab too.
	app.openRequest(sampleRequest(tt, "Get user"))
	assertTabs(tt, app, "List users, /Get user/")
	assertActive(tt, app, "Get user")
}

func TestBlankTabInViewBecomesThePreview(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	app.newTab()
	app.openRequest(sampleRequest(tt, "Get user"))
	// There's only ever one preview: the old one closes.
	assertTabs(tt, app, "/Get user/")
}

func TestCommittingToThePreviewKeepsIt(tt *testing.T) {
	for _, tc := range []struct {
		name   string
		commit func(*App)
	}{
		{"edit", func(a *App) { a.current().url.SetText("http://localhost:8000/users?page=2"); a.current().urlEdited() }},
		{"method", func(a *App) { a.setMethod(model.MethodPost) }},
		{"save", func(a *App) { a.saveRequest() }},
		{"send", func(a *App) {
			a.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) { return fixedResponse(), nil })
			a.current().dispatch = func(func()) {}
			a.send()
		}},
		{"curl", func(a *App) { a.importCurl("curl http://localhost:8000/health") }},
		{"command", func(a *App) { a.keepSession(a.active.Peek()) }},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := testApp()
			app.openRequest(sampleRequest(tt, "List users"))
			tc.commit(app)
			app.openRequest(sampleRequest(tt, "Get user"))
			if got := len(app.sessions.Peek()); got != 2 {
				tt.Fatalf("%d tabs open, want the kept List users and a preview of Get user (%s)", got, tabTitles(app))
			}
			if first := app.sessions.Peek()[0]; first.preview.Peek() || first.file.Peek() != "users/list-users.posting.yaml" {
				tt.Errorf("the first tab is %s, want List users kept open", tabTitles(app))
			}
		})
	}
}

func TestPreviewNeverHoldsUnsavedChanges(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.body.SetText("{}")
	s.touch()
	if s.preview.Peek() || !s.dirty.Peek() {
		tt.Fatal("editing the preview left it the preview")
	}
	app.openRequest(sampleRequest(tt, "Get user"))
	if s.body.GetText() != "{}" || !s.dirty.Peek() {
		tt.Error("opening another request replaced unsaved changes")
	}
}

func TestDeletingThePreviewsRequestKeepsTheTab(tt *testing.T) {
	app := testApp()
	req := sampleRequest(tt, "List users")
	app.openRequest(req)
	app.deleteRequests([]string{req.File})
	// The tab now holds the only copy of the request.
	app.openRequest(sampleRequest(tt, "Get user"))
	assertTabs(tt, app, "List users, /Get user/")
}

func TestClosingThePreview(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	app.keepSession(app.active.Peek())
	app.openRequest(sampleRequest(tt, "Get user"))
	app.closeSession(app.active.Peek())
	assertTabs(tt, app, "List users")
	app.openRequest(sampleRequest(tt, "Login"))
	assertTabs(tt, app, "List users, /Login/")
}

func TestNewAndDuplicatedRequestsArePermanent(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	app.duplicateRequests([]model.Request{sampleRequest(tt, "List users")})
	app.newTab()
	for _, s := range app.sessions.Peek()[1:] {
		if s.preview.Peek() {
			tt.Errorf("%s opened as the preview", s.title.Peek())
		}
	}
}

func TestHistoryOpensInThePreview(tt *testing.T) {
	app := testApp()
	var entries []model.HistoryEntry
	for i, name := range []string{"List users", "Get user"} {
		entries = append(entries, sentEntry(int64(i+1), sampleRequest(tt, name), fixedResponse(), time.Now()))
	}
	app.openHistory(entries[0])
	app.openHistory(entries[1])
	assertTabs(tt, app, "/Get user/")
	if app.current().fromHistory.Peek() == nil {
		tt.Error("the history entry's response isn't shown")
	}
	// A request from the collection replaces a history preview too.
	app.openRequest(sampleRequest(tt, "Login"))
	assertTabs(tt, app, "/Login/")
}

func TestKeepTabCommandOnlyForThePreview(tt *testing.T) {
	hasKeep := func(app *App) bool {
		for _, item := range app.paletteItems() {
			if item.Label == "Keep tab open" {
				return true
			}
		}
		return false
	}
	app := testApp()
	if hasKeep(app) {
		tt.Error("a blank tab offers to be kept open")
	}
	app.openRequest(sampleRequest(tt, "List users"))
	if !hasKeep(app) {
		tt.Fatal("the preview tab doesn't offer to be kept open")
	}
	for _, item := range app.paletteItems() {
		if item.Label == "Keep tab open" {
			item.Action()
		}
	}
	if app.current().preview.Peek() {
		tt.Error("the command didn't keep the tab")
	}
}

// clickAt presses the mouse at (x, y) as the app's mouse routing does: the
// widget under the pointer and the collection that owns it both hear of it,
// and then the widget under the pointer is clicked.
func (s *screen) clickAt(tt *testing.T, x, y, count int) {
	tt.Helper()
	entry := s.renderer.WidgetAt(x, y)
	if entry == nil {
		tt.Fatalf("nothing at (%d,%d)", x, y)
	}
	event := func(e *t.WidgetEntry) t.MouseEvent {
		return t.MouseEvent{X: x, Y: y, LocalX: x - e.Bounds.X, LocalY: y - e.Bounds.Y, Button: uv.MouseLeft, ClickCount: count, WidgetID: e.ID}
	}
	if handler, ok := entry.EventWidget.(t.MouseDownHandler); ok {
		handler.OnMouseDown(event(entry))
	}
	if owner := s.renderer.PointerOwnerAt(x, y); owner != nil && owner != entry {
		if handler, ok := owner.EventWidget.(t.MouseDownHandler); ok {
			handler.OnMouseDown(event(owner))
		}
	}
	if clickable, ok := entry.EventWidget.(t.Clickable); ok {
		clickable.OnClick(event(entry))
	}
	s.render()
}

// findText is where text first appears in screen, between columns from
// and to.
func findText(tt *testing.T, screen, text string, from, to int) (x, y int) {
	tt.Helper()
	width := len([]rune(text))
	for y, line := range strings.Split(screen, "\n") {
		cells := []rune(line)
		for x := from; x+width <= min(to, len(cells)); x++ {
			if string(cells[x:x+width]) == text {
				return x, y
			}
		}
	}
	tt.Fatalf("%q isn't on screen:\n%s", text, screen)
	return 0, 0
}

// inTree is where the collection shows the request named name.
func (s *screen) inTree(tt *testing.T, name string) (x, y int) {
	tt.Helper()
	return findText(tt, s.renderer.ScreenText(), name, 0, s.strip().X)
}

// inStrip is where the strip of open tabs shows title.
func (s *screen) inStrip(tt *testing.T, title string) (x, y int) {
	tt.Helper()
	return findText(tt, s.renderer.ScreenText(), title, s.strip().X, snapW)
}

func TestDoubleClickingARequestKeepsIt(tt *testing.T) {
	app := testApp()
	s := newScreen(app, snapW, snapH)
	x, y := s.inTree(tt, "List users")
	s.clickAt(tt, x, y, 1)
	assertTabs(tt, app, "/List users/")
	s.clickAt(tt, x, y, 2)
	assertTabs(tt, app, "List users")

	// A single click still previews the next request beside it.
	x, y = s.inTree(tt, "Get user")
	s.clickAt(tt, x, y, 1)
	assertTabs(tt, app, "List users, /Get user/")
}

func TestDoubleClickingThePreviewTabKeepsIt(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := newScreen(app, snapW, snapH)
	x, y := s.inStrip(tt, "List users")
	s.clickAt(tt, x, y, 1)
	assertTabs(tt, app, "/List users/")
	s.clickAt(tt, x, y, 2)
	assertTabs(tt, app, "List users")
}

func TestPreviewIsItalic(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := newScreen(app, snapW, snapH)
	tabX, tabY := s.inStrip(tt, "List users")
	treeX, treeY := s.inTree(tt, "List users")
	italic := func() (tab, tree bool) {
		buf := uv.NewBuffer(snapW, snapH)
		t.NewRenderer(buf, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil)).Render(app)
		at := func(x, y int) bool { return buf.CellAt(x, y).Style.Attrs&uv.AttrItalic != 0 }
		return at(tabX, tabY), at(treeX, treeY)
	}
	if tab, tree := italic(); !tab || !tree {
		tt.Fatalf("italic tab = %v, request in the collection = %v; want both", tab, tree)
	}
	app.keepSession(app.active.Peek())
	if tab, tree := italic(); tab || tree {
		tt.Errorf("a kept tab is still in italics (tab %v, collection %v)", tab, tree)
	}
}
