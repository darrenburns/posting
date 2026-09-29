package ui

import (
	"fmt"
	"strings"
	"testing"

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

// tabRow is the text of the strip of open tabs as last painted.
func (s *screen) tabRow() string {
	return strings.Split(s.renderer.ScreenText(), "\n")[s.app.sessionView.y]
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
	wheel := func(scroll func(*t.Scrollable) bool) {
		tt.Helper()
		scrollables := s.renderer.ScrollablesAt(view.x+10, view.y)
		if len(scrollables) == 0 || !scroll(scrollables[0]) {
			tt.Fatal("the wheel over the strip isn't handled")
		}
		s.render()
	}
	wheel(func(sc *t.Scrollable) bool { return sc.ScrollDown(1) })
	wheel(func(sc *t.Scrollable) bool { return sc.ScrollDown(1) })
	if view.first != 2 {
		tt.Fatalf("two wheel notches should scroll two tabs; first shown = %d", view.first)
	}
	wheel(func(sc *t.Scrollable) bool { return sc.ScrollUp(1) })
	if view.first != 1 {
		tt.Fatalf("wheeling up should scroll back a tab; first shown = %d", view.first)
	}
	for range 30 {
		wheel(func(sc *t.Scrollable) bool { return sc.ScrollRight(1) })
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
	for _, width := range []int{snapW - 40, snapW - 60, snapW - 20} {
		// The first frame at a new width measures the strip, and the
		// second (forced, as in newScreen) lays the tabs out to fit it.
		s.renderer.Resize(width, snapH)
		s.render()
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

	// It drops down from the right end of the strip.
	view := app.sessionView
	float := s.renderer.TopFloat()
	if float == nil || float.Y != view.y+1 || float.X+float.Width != view.x+view.width {
		tt.Fatalf("the tab search should hang below the right end of the strip (%d,%d w%d); float = %+v", view.x, view.y, view.width, float)
	}

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
	app.current().dirty.Set(true)
	app.openTabSearch()
	t.AssertSnapshot(tt, app, snapW, snapH, "The open-tab search dropped down from the right end of the tab strip, on the active (unsaved) Request 12")
}
