package ui

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// openTabs opens n more request tabs, named "Request 01" onwards, leaving
// the last one active.
func openTabs(app *App, n int) {
	for i := 1; i <= n; i++ {
		req := model.NewRequest()
		req.Name = fmt.Sprintf("Request %02d", i)
		app.openSession(req)
	}
}

// strip is where the strip of open tabs was last painted.
func (s *screen) strip() t.Rect {
	return s.renderer.WidgetByID(sessionTabsID).Visible
}

// tabRow is the text of the strip of open tabs as last painted.
func (s *screen) tabRow() string {
	return strings.Split(s.renderer.ScreenText(), "\n")[s.strip().Y]
}

// wheelTabs turns the wheel over the strip, ten cells in, and paints the
// result. It reports whether the strip took the wheel. The strip is a plain
// row: nothing under the pointer scrolls, and the wheel bubbles up from
// the tab under it to the strip.
func (s *screen) wheelTabs(tt *testing.T, button uv.MouseButton) bool {
	tt.Helper()
	strip := s.renderer.WidgetByID(sessionTabsID)
	x, y := strip.Visible.X+10, strip.Visible.Y
	if under := s.renderer.WidgetAt(x, y); under == nil || !strip.Visible.Contains(under.Visible.X, under.Visible.Y) {
		tt.Fatalf("the pointer at (%d,%d) isn't over the strip", x, y)
	}
	if n := len(s.renderer.ScrollablesAt(x, y)); n > 0 {
		tt.Fatalf("the strip sits in %d scrollables", n)
	}
	handler, ok := strip.EventWidget.(t.MouseWheelHandler)
	if !ok {
		tt.Fatalf("the strip (%T) doesn't handle the wheel", strip.EventWidget)
	}
	handled := handler.OnMouseWheel(t.MouseEvent{X: x, Y: y, LocalX: x - strip.Bounds.X, Button: button})
	s.render()
	return handled
}

// showTab switches to the tab titled title the way the keyboard does, one
// tab at a time, and paints the result.
func (s *screen) showTab(tt *testing.T, title string) {
	tt.Helper()
	for range s.app.sessions.Peek() {
		if s.app.current().title.Peek() == title {
			s.render()
			return
		}
		s.app.cycleSession(1)
	}
	tt.Fatalf("no tab titled %q", title)
}

func TestSessionTabsMarkOverflowOnlyWhenTabsAreHidden(tt *testing.T) {
	app := testApp()
	openTabs(app, 2)
	s := newScreen(app, snapW, snapH)
	if row := s.tabRow(); strings.ContainsAny(row, "‹›") {
		tt.Fatalf("three tabs fit, but the strip shows overflow marks:\n%s", row)
	}

	openTabs(app, 10)
	s.render()
	row := s.tabRow()
	if !strings.Contains(row, "‹") || strings.Contains(row, "›") {
		tt.Fatalf("scrolled to the last tab, only ‹ should show:\n%s", row)
	}

	s.showTab(tt, "Untitled")
	row = s.tabRow()
	if strings.Contains(row, "‹") || !strings.Contains(row, "›") {
		tt.Fatalf("scrolled to the first tab, only › should show:\n%s", row)
	}

	s.showTab(tt, "Request 06")
	row = s.tabRow()
	if !strings.Contains(row, "‹") || !strings.Contains(row, "›") {
		tt.Fatalf("scrolled to the middle, both marks should show:\n%s", row)
	}
}

func TestSessionTabsKeepTheActiveTabInView(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	for range app.sessions.Peek() {
		app.cycleSession(1)
		s.render()
		if title := app.current().title.Peek(); !strings.Contains(s.tabRow(), title) {
			tt.Fatalf("the active tab %q is out of view:\n%s", title, s.tabRow())
		}
	}
	for range app.sessions.Peek() {
		app.cycleSession(-1)
		s.render()
		if title := app.current().title.Peek(); !strings.Contains(s.tabRow(), title) {
			tt.Fatalf("going back, the active tab %q is out of view:\n%s", title, s.tabRow())
		}
	}
}

func TestSessionTabsScrollWithTheArrowsAndWheel(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	s.showTab(tt, "Untitled")
	active := app.active.Peek()

	clickText(tt, app, "›", 0)
	s.render()
	if app.sessionView.first != 1 {
		tt.Fatalf("clicking › should scroll a tab along; first shown = %d", app.sessionView.first)
	}
	if strings.Contains(s.tabRow(), "Untitled") || app.active.Peek() != active {
		tt.Fatalf("scrolling should hide the first tab without switching to another:\n%s", s.tabRow())
	}
	clickText(tt, app, "‹", 0)
	s.render()
	if app.sessionView.first != 0 {
		tt.Fatalf("clicking ‹ should scroll back; first shown = %d", app.sessionView.first)
	}

	view := app.sessionView
	wheel := func(button uv.MouseButton) {
		tt.Helper()
		if !s.wheelTabs(tt, button) {
			tt.Fatal("the wheel over the strip isn't handled")
		}
	}
	wheel(uv.MouseWheelDown)
	wheel(uv.MouseWheelDown)
	if view.first != 2 {
		tt.Fatalf("two wheel notches should scroll two tabs; first shown = %d", view.first)
	}
	wheel(uv.MouseWheelUp)
	if view.first != 1 {
		tt.Fatalf("wheeling up should scroll back a tab; first shown = %d", view.first)
	}
	for range 30 {
		wheel(uv.MouseWheelRight)
	}
	if row := s.tabRow(); strings.Contains(row, "›") || !strings.Contains(row, "Request 12") {
		tt.Fatalf("scrolling past the end should stop at the last tab:\n%s", row)
	}

	// Switching tabs brings the active one back into view.
	app.cycleSession(1)
	s.render()
	if !strings.Contains(s.tabRow(), "Request 01") {
		tt.Fatalf("switching tabs should scroll to the new one:\n%s", s.tabRow())
	}
}

func TestSessionTabsKeepTheActiveTabInViewWhenResized(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	// Stepping along from the first tab leaves Request 06 at the right edge.
	s.showTab(tt, "Untitled")
	for range 6 {
		app.cycleSession(1)
		s.render()
	}
	// One frame at a new width, narrower or wider than the screen started,
	// lays the tabs out to fit it.
	for _, width := range []int{snapW - 40, snapW - 60, snapW + 40, snapW - 20} {
		s.renderer.Resize(width, snapH)
		s.render()
		if row := s.tabRow(); !strings.Contains(row, "Request 06") {
			tt.Fatalf("at width %d the active tab is out of view:\n%s", width, row)
		}
		if view := app.sessionView; view.first > view.maxFirst {
			tt.Fatalf("at width %d the strip is scrolled past its end: first %d > %d", width, view.first, view.maxFirst)
		}
	}
}

func TestSessionTabsListOnlyWithMoreThanOneTab(tt *testing.T) {
	app := testApp()
	app.current().title.Set(strings.Repeat("A long request name ", 4))
	s := newScreen(app, 30, snapH)
	if row := s.tabRow(); strings.Contains(row, "▾") {
		tt.Fatalf("a single tab that overflows shouldn't offer the tab list:\n%s", row)
	}
}

func TestSessionTabsArePlainRow(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	s.showTab(tt, "Request 06")
	strip := s.renderer.WidgetByID(sessionTabsID)
	if strip.Bounds.Height != 1 {
		tt.Fatalf("the strip is %d rows tall", strip.Bounds.Height)
	}
	// No hidden scrollbar column: the tab list button ends the strip.
	row := []rune(s.tabRow())
	if end := strip.Visible.X + strip.Visible.Width; string(row[end-8:end]) != " › +  ▾ " {
		tt.Fatalf("the strip doesn't end with its marks and buttons:\n%s", string(row))
	}
	for x := strip.Visible.X; x < strip.Visible.X+strip.Visible.Width; x++ {
		if len(s.renderer.ScrollablesAt(x, strip.Visible.Y)) > 0 {
			tt.Fatalf("column %d of the strip is in a scrollable", x)
		}
	}
	// A strip with every tab in view leaves the wheel be.
	few := testApp()
	openTabs(few, 1)
	if newScreen(few, snapW, snapH).wheelTabs(tt, uv.MouseWheelDown) {
		tt.Error("a strip with nothing hidden took the wheel")
	}
}

func TestTabSearchFiltersAndSwitchesTabs(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	s.showTab(tt, "Untitled")
	if keys := app.keysFor("search-tabs"); len(keys) == 0 || keys[0] != "alt+down" {
		tt.Fatalf("search-tabs keys = %v", keys)
	}

	clickText(tt, app, " ▾ ", 0)
	if !app.tabSearch.Visible.Peek() {
		tt.Fatal("clicking ▾ should open the tab search")
	}
	if item, ok := app.tabSearch.CurrentItem(); !ok || item.Label != "Untitled" {
		tt.Fatalf("the search should start on the active tab, not %q", item.Label)
	}
	s.render()

	// It drops down from the right end of the strip, and follows the strip
	// when the screen grows or the sidebar goes.
	hangsFromStrip := func(when string) {
		tt.Helper()
		strip, float := s.strip(), s.renderer.TopFloat()
		if float == nil || float.Y != strip.Y+1 || float.X+float.Width != strip.X+strip.Width {
			tt.Fatalf("%s, the tab search should hang below the right end of the strip %+v; float = %+v", when, strip, float)
		}
	}
	hangsFromStrip("at first")
	// There's no title row: the search input is the first thing in it, with
	// only its own padding above.
	if float := s.renderer.TopFloat(); float != nil {
		row := strings.Split(s.renderer.ScreenText(), "\n")[float.Y+1]
		if !strings.Contains(row, "Search open tabs…") {
			tt.Fatalf("the search input should open the tab search, not %q", row)
		}
	}
	s.renderer.Resize(snapW+30, snapH)
	s.render()
	hangsFromStrip("on a wider screen")
	app.sidebarVisible.Set(false)
	s.render()
	hangsFromStrip("without the sidebar")

	input := s.paletteInput(tt, tabSearchID+"-input")
	input.State.SetText("uest 07")
	input.OnChange("uest 07")
	if item, ok := app.tabSearch.CurrentItem(); !ok || item.Label != "Request 07" {
		tt.Fatalf("filtering should put Request 07 first, not %q", item.Label)
	}
	for _, kb := range input.ExtraKeybinds {
		if kb.Key == "enter" {
			kb.Action()
		}
	}
	if app.tabSearch.Visible.Peek() {
		tt.Fatal("choosing a tab should close the search")
	}
	if got := app.current().title.Peek(); got != "Request 07" {
		tt.Fatalf("active tab = %q, want Request 07", got)
	}
	s.render()
	if !strings.Contains(s.tabRow(), "Request 07") {
		tt.Fatalf("the chosen tab should be scrolled into view:\n%s", s.tabRow())
	}
}

func TestTabSearchRevealsTheActiveTabScrolledAway(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	s := newScreen(app, snapW, snapH)
	app.sessionView.scroll(-20)
	s.render()
	if strings.Contains(s.tabRow(), "Request 12") {
		tt.Fatal("the strip should have scrolled away from the active tab")
	}
	app.openTabSearch()
	item, _ := app.tabSearch.CurrentItem()
	item.Action()
	s.render()
	if !strings.Contains(s.tabRow(), "Request 12") {
		tt.Fatalf("choosing the active tab should scroll back to it:\n%s", s.tabRow())
	}
}

// paletteInput finds a palette's search input on screen.
func (s *screen) paletteInput(tt *testing.T, id string) t.TextInput {
	tt.Helper()
	for _, entry := range s.renderer.Update(s.app) {
		if entry.ID != id {
			continue
		}
		input, ok := entry.Focusable.(t.TextInput)
		if !ok {
			tt.Fatalf("%s is a %T", id, entry.Focusable)
		}
		return input
	}
	tt.Fatalf("%s is not on screen", id)
	return t.TextInput{}
}

func TestSnapshotSessionTabsOverflow(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	app.showSession(app.sessions.Peek()[6].id)
	t.AssertSnapshot(tt, app, snapW, snapH, "More open tabs than fit: the strip has scrolled to the active Request 06, with ‹ and › marking hidden tabs either side and ▾ at the right end")
}

func TestSnapshotTabSearch(tt *testing.T) {
	app := testApp()
	openTabs(app, 12)
	for _, s := range app.sessions.Peek() {
		switch title := s.title.Peek(); title {
		case "Untitled", "Request 09":
		default:
			s.file.Set("users/" + strings.ToLower(strings.ReplaceAll(title, " ", "-")) + ".posting.yaml")
		}
	}
	app.current().dirty.Set(true)
	app.openTabSearch()
	t.AssertSnapshot(tt, app, snapW, snapH, "The open-tab search dropped down from the right end of the tab strip, on the active Request 12 with unsaved changes. Saved tabs show their path on a second line; Untitled and Request 09 have no file and take one line")
}
