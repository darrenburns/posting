package ui

import (
	"fmt"
	"slices"
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

func TestSessionTabHover(tt *testing.T) {
	app := homeApp("aurora")
	openTabs(app, 1)
	p := t.NewPilot(tt, app, snapW, snapH)
	theme, _ := t.GetTheme("aurora")
	bounds, _ := p.Bounds(sessionTabsID)
	for _, title := range []string{"Untitled", "Request 01"} {
		x, y := findText(tt, p.ScreenText(), title, bounds.X, bounds.X+bounds.Width)
		row := []rune(strings.Split(p.ScreenText(), "\n")[y])
		closeX := x + len(title)
		for row[closeX] != '✕' {
			closeX++
		}
		base := t.FromANSI(p.Buffer().CellAt(x, y).Style.Bg)
		tinted := base
		if title == "Untitled" {
			tinted = theme.Hover.BlendOver(base)
		}
		for _, target := range []int{x, closeX, x, 0} {
			p.MouseMove(target, y)
			want := tinted
			if target == 0 {
				want = base
			}
			// Method badge, title, marker, close control and padding share the tint.
			for cellX := x - 5; cellX <= closeX+1; cellX++ {
				if got := t.FromANSI(p.Buffer().CellAt(cellX, y).Style.Bg); got != want {
					tt.Fatalf("%s background at %d = %s, want %s (pointer x=%d)", title, cellX, got.Hex(), want.Hex(), target)
				}
			}
			closeFG := theme.TextMuted
			if target == closeX {
				closeFG = theme.Text
			}
			if got := t.FromANSI(p.Buffer().CellAt(closeX, y).Style.Fg); got != closeFG {
				tt.Fatalf("%s close foreground = %s, want %s", title, got.Hex(), closeFG.Hex())
			}
			assertActive(tt, app, "Request 01")
		}
	}
	// A hovered close button still closes without selecting the inactive tab.
	x, y := findText(tt, p.ScreenText(), "✕", bounds.X, bounds.X+bounds.Width)
	p.MouseMove(x, y)
	p.ClickAt(x, y)
	assertTabs(tt, app, "Request 01")
}

func TestSessionTabsDragToReorder(tt *testing.T) {
	for _, tc := range []struct {
		name     string
		n        int
		from, to int
		order    []int
	}{
		{"right", 2, 0, 2, []int{1, 2, 0}},
		{"left", 2, 2, 0, []int{2, 0, 1}},
		{"overflow-left", 12, 12, 11, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 11}},
		{"overflow-right", 12, 11, 12, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 11}},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := testApp()
			openTabs(app, tc.n)
			before := slices.Clone(app.sessions.Peek())
			before[tc.from].preview.Set(true)
			p := t.NewPilot(tt, app, snapW, snapH)
			bounds, _ := p.Bounds(sessionTabsID)
			row := strings.Split(p.ScreenText(), "\n")[bounds.Y]
			x := func(index int) int {
				tt.Helper()
				at := strings.Index(row, before[index].title.Peek())
				if at < 0 {
					tt.Fatalf("tab %d not visible: %s", index, row)
				}
				return len([]rune(row[:at]))
			}
			p.MouseDown(x(tc.from), bounds.Y, uv.MouseLeft, 0)
			p.MouseMove(x(tc.to), bounds.Y)
			if !slices.Equal(app.sessions.Peek(), before) {
				tt.Fatal("dragging changed the order before release")
			}
			p.MouseUp(x(tc.to), bounds.Y, uv.MouseLeft, 0)
			var want []*Session
			for _, index := range tc.order {
				want = append(want, before[index])
			}
			if !slices.Equal(app.sessions.Peek(), want) {
				tt.Fatalf("drop did not reorder the existing sessions: %s", tabTitles(app))
			}
			if app.current() != before[tc.from] {
				tt.Fatal("dropping switched away from the dragged request")
			}
			if !app.current().preview.Peek() {
				tt.Fatal("dragging changed the preview state")
			}
			row = strings.Split(p.ScreenText(), "\n")[bounds.Y]
			if strings.Index(row, want[tc.to].title.Peek()) < 0 {
				tt.Fatalf("dragged tab disappeared after release: %s", row)
			}
			// Keyboard navigation must follow the new order, not the old one.
			p.Press("alt+right")
			if app.current() != want[(tc.to+1)%len(want)] {
				tt.Fatal("next-tab did not follow the dropped tab's new position")
			}
		})
	}
}

func TestSessionTabDragCancellationAndClose(tt *testing.T) {
	for _, end := range []string{"escape", "outside", "same-tab", "close"} {
		tt.Run(end, func(tt *testing.T) {
			app := testApp()
			openTabs(app, 2)
			p := t.NewPilot(tt, app, snapW, snapH)
			bounds, _ := p.Bounds(sessionTabsID)
			x, y := findText(tt, p.ScreenText(), "Request 02", bounds.X, bounds.X+bounds.Width)
			target, _ := findText(tt, p.ScreenText(), "Request 01", bounds.X, bounds.X+bounds.Width)
			if end == "close" {
				row := []rune(strings.Split(p.ScreenText(), "\n")[y])
				for row[x] != '✕' {
					x++
				}
			}
			p.MouseDown(x, y, uv.MouseLeft, 0)
			p.MouseMove(target, y)
			switch end {
			case "escape":
				p.Press("escape")
			case "outside":
				y += 3
			case "same-tab":
				target = x + 1
			}
			p.MouseUp(target, y, uv.MouseLeft, 0)
			if end == "close" {
				assertTabs(tt, app, "Untitled, Request 01")
			} else {
				assertTabs(tt, app, "Untitled, Request 01, Request 02")
			}
		})
	}
}

func TestSessionTabsUnderlineOnlyTheActiveTitle(tt *testing.T) {
	// Aurora's cursor and accent colours differ.
	app := homeApp("aurora")
	openTabs(app, 1)
	p := t.NewPilot(tt, app, snapW, snapH)
	theme, _ := t.GetTheme("aurora")
	bounds, _ := p.Bounds(sessionTabsID)
	for _, active := range []string{"Request 01", "Untitled"} {
		row := strings.Split(p.ScreenText(), "\n")[bounds.Y]
		position := func(title string) int {
			index := strings.Index(row, title)
			if index < 0 {
				tt.Fatalf("tab %q is missing from %q", title, row)
			}
			return len([]rune(row[:index]))
		}
		p.ClickAt(position(active), bounds.Y)
		for _, title := range []string{"Untitled", "Request 01"} {
			x := position(title)
			for i := range len(title) {
				style := p.Buffer().CellAt(x+i, bounds.Y).Style
				if title == active {
					if style.Attrs&uv.AttrBold == 0 || style.Underline != uv.UnderlineSingle || t.FromANSI(style.UnderlineColor) != theme.ActiveCursor {
						tt.Fatalf("active tab %q should be bold with a cursor-coloured underline", title)
					}
				} else if style.Underline != uv.UnderlineNone {
					tt.Fatalf("inactive tab %q should not be underlined", title)
				}
			}
			for _, pad := range []int{x - 1, x + len(title)} {
				if p.Buffer().CellAt(pad, bounds.Y).Style.Underline != uv.UnderlineNone {
					tt.Fatal("tab padding should not be underlined")
				}
			}
		}
	}
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
	// Saved requests have paths long enough to fill the dropdown on their
	// own. They must not hide the title users search for.
	app.sessions.Peek()[7].file.Set("/home/user/projects/api-collection/billing/invoices.posting.yaml")
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
	for _, query := range []string{"uest 07", "INVOICES", "billing"} {
		input.State.SetText(query)
		input.OnChange(query)
		if item, ok := app.tabSearch.CurrentItem(); !ok || item.Label != "Request 07" {
			tt.Errorf("filtering by %q should put Request 07 first, not %q", query, item.Label)
		}
		s.render()
		float := s.renderer.TopFloat()
		var rows []string
		for _, row := range strings.Split(s.renderer.ScreenText(), "\n")[float.Y : float.Y+float.Height] {
			rows = append(rows, string([]rune(row)[float.X:float.X+float.Width]))
		}
		text := strings.Join(rows, "\n")
		if !strings.Contains(text, "GET Request 07") || !strings.Contains(text, "invoices.posting.yaml") || strings.Contains(text, "Request 08") {
			tt.Errorf("searching %q should show the matching title, method and filename, without other tabs:\n%s", query, text)
		}
		for i, row := range rows {
			if strings.Contains(row, "GET Request 07") {
				if strings.Contains(row, "posting.yaml") || i+1 == len(rows) || !strings.Contains(rows[i+1], "invoices.posting.yaml") {
					tt.Errorf("the path should be on the line below the title:\n%s", text)
				}
			}
		}
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
	app.sessions.Peek()[10].file.Set("requests/users.posting.yaml")
	app.sessions.Peek()[11].file.Set("/home/user/projects/api-collection/users/create.posting.yaml")
	app.current().file.Set("/home/user/projects/api-collection/billing/invoices.posting.yaml")
	app.current().dirty.Set(true)
	app.openTabSearch()
	t.AssertSnapshot(tt, app, snapW, snapH, "The open-tab search shows file paths below titles, on the active (unsaved) Request 12; tabs without paths stay on one line")
}
