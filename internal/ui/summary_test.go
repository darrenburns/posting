package ui

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/config"
	"github.com/darrenburns/posting/internal/model"
)

// moveTreeCursor puts the tree cursor on the request with the given name.
func moveTreeCursor(tt *testing.T, app *App, name string) {
	tt.Helper()
	app.tree.CursorPath.Set(requestPath(tt, app, name))
}

// requestPath is the tree path of the request with the given name.
func requestPath(tt *testing.T, app *App, name string) []int {
	tt.Helper()
	var find func(nodes []t.TreeNode[treeItem], prefix []int) []int
	find = func(nodes []t.TreeNode[treeItem], prefix []int) []int {
		for i, node := range nodes {
			path := append(slices.Clip(prefix), i)
			if r := node.Data.Request; r != nil && r.Name == name {
				return path
			}
			if found := find(node.Children, path); found != nil {
				return found
			}
		}
		return nil
	}
	path := find(app.tree.Nodes.Peek(), nil)
	if path == nil {
		tt.Fatalf("no request named %q in the tree", name)
	}
	return path
}

// appWithDescription is the sample app with one request's description
// replaced, and its settings changed by change if it is set.
func appWithDescription(name, description string, change func(*config.Settings)) *App {
	settings := config.Defaults()
	if change != nil {
		change(&settings)
	}
	collection := model.SampleCollection()
	for _, child := range collection.Children {
		for i := range child.Requests {
			if child.Requests[i].Name == name {
				child.Requests[i].Description = description
			}
		}
	}
	return New(Config{
		Version:      "3.0.0-dev",
		Collection:   collection,
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
		Settings:     &settings,
	})
}

const longDescription = "Fetch a page of users, newest first.\n\n" +
	"Supports `page` and `per_page` query parameters, and filtering by `role`. " +
	"Deactivated users are left out unless `include_inactive` is set."

func TestSnapshotRequestSummary(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	moveTreeCursor(tt, app, "List users")
	t.RequestFocus(treeID)
	t.AssertSnapshot(tt, app, snapW, snapH, "Collection focused: the highlighted request's summary floats beside its row")
}

func TestSnapshotRequestSummaryOpensUpwards(tt *testing.T) {
	app := appWithDescription("Delete user", longDescription, nil)
	moveTreeCursor(tt, app, "Delete user")
	t.RequestFocus(treeID)
	t.AssertSnapshot(tt, app, snapW, 18, "Near the bottom of a short terminal, the summary opens upwards from the row")
}

func TestSnapshotRequestSummaryCollectionOnRight(tt *testing.T) {
	app := appWithDescription("List users", longDescription, func(s *config.Settings) {
		s.CollectionBrowser.Position = "right"
	})
	moveTreeCursor(tt, app, "List users")
	t.RequestFocus(treeID)
	t.AssertSnapshot(tt, app, snapW, snapH, "Collection on the right: the summary floats to the left of the row")
}

// summary checks that the summary bubble is on screen beside the tree
// cursor's row (see summaryOf) and returns its bounds.
func (s *screen) summary(tt *testing.T) t.Rect {
	tt.Helper()
	return s.summaryOf(tt, s.app.tree.CursorPath.Peek())
}

// summaryOf checks that the summary bubble is on screen beside the tree row
// at path, with its title (or, when it opens upwards, its last line) on the
// row, and returns its bounds. Beside a tree on the left, it starts just past
// the labels of the rows it covers, hiding none of them; beside one on the
// right, it stays off the tree.
func (s *screen) summaryOf(tt *testing.T, path []int) t.Rect {
	tt.Helper()
	bubble, viewport := s.renderer.WidgetByID(summaryID), s.renderer.WidgetByID(treeViewportID)
	if bubble == nil || bubble.Visible.IsEmpty() {
		tt.Fatalf("no summary beside the tree row %v", path)
	}
	row, _ := s.app.treeRow(path)
	want := viewport.Bounds.Y + row - s.app.treeScroll.GetOffset()
	b := bubble.Bounds
	if title, last := b.Y+1, b.Y+b.Height-2; title != want && last != want {
		tt.Fatalf("summary spans rows %d-%d, want its title or last line beside the row on row %d", title, last, want)
	}
	tree := viewport.Bounds
	if s.app.settings.CollectionBrowser.Position == "right" {
		if right := b.X + b.Width; right >= tree.X {
			tt.Fatalf("summary ends at column %d, over the tree, which starts at %d", right, tree.X)
		}
		return b
	}
	if b.X <= tree.X {
		tt.Fatalf("summary starts at column %d, before the tree at %d", b.X, tree.X)
	}
	if b.X-1 >= tree.X+tree.Width {
		// Past a label running to the edge of the tree, it clears the
		// divider.
		return b
	}
	lines := strings.Split(s.renderer.ScreenText(), "\n")
	tight := false
	for y := max(b.Y, tree.Y); y < min(b.Y+b.Height, tree.Y+tree.Height); y++ {
		cells := []rune(lines[y])
		if cells[b.X-1] != ' ' {
			tt.Fatalf("the label on row %d runs under the summary at column %d: %q", y, b.X, string(cells[tree.X:b.X]))
		}
		tight = tight || cells[b.X-2] != ' '
	}
	if !tight {
		tt.Fatalf("the summary at column %d leaves more than a cell after the labels beside it", b.X)
	}
	return b
}

// summaryTitle is the title line of the summary on screen.
func (s *screen) summaryTitle(tt *testing.T) string {
	tt.Helper()
	b := s.renderer.WidgetByID(summaryID).Bounds
	line := []rune(strings.Split(s.renderer.ScreenText(), "\n")[b.Y+1])
	return strings.TrimSpace(string(line[b.X+1 : b.X+b.Width-1]))
}

func TestSummaryShowsBesideTheFocusedTreeCursor(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Fatal("the summary shows while the tree isn't focused")
	}
	// It is there on the frame that focuses the tree.
	s.focusID(tt, treeID)
	s.summary(tt)
	viewport := s.renderer.WidgetByID(treeViewportID)

	// Requests without a description have no bubble, and the tree keeps its
	// height either way.
	moveTreeCursor(tt, app, "Update user")
	s.render()
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Error("a request without a description has a summary")
	}
	if got := s.renderer.WidgetByID(treeViewportID).Bounds; got != viewport.Bounds {
		tt.Errorf("the tree resized from %v to %v as the cursor moved", viewport.Bounds, got)
	}
}

func TestSummaryFollowsTheCursorResizeAndScroll(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)
	first := s.summary(tt)

	// The frame after the cursor moves has the bubble beside the new row.
	moveTreeCursor(tt, app, "Login")
	s.render()
	if moved := s.summary(tt); moved.Y >= first.Y {
		tt.Errorf("the summary stayed at row %d after the cursor moved up", moved.Y)
	}

	// Resizing lays it out for the new size in one frame, including past
	// the size the screen started at. Too narrow a workspace has no bubble.
	for _, width := range []int{snapW + 40, snapW - 30} {
		s.renderer.Resize(width, snapH+10)
		s.render()
		if bubble := s.summary(tt); bubble.X+bubble.Width > width {
			tt.Errorf("at width %d the summary runs off screen: %v", width, bubble)
		}
	}
	s.renderer.Resize(50, snapH)
	s.render()
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Error("the summary squeezed into a workspace too narrow to read it")
	}

	// Scrolling the tree of a short screen moves the bubble with its row,
	// and a row scrolled out of view has none.
	s.renderer.Resize(snapW, 10)
	s.render()
	before := s.summary(tt)
	app.treeScroll.SetOffset(1)
	s.render()
	if after := s.summary(tt); after.Y != before.Y-1 {
		tt.Errorf("scrolling the tree a row moved the summary from row %d to %d", before.Y, after.Y)
	}
	row, _ := app.treeRow(app.tree.CursorPath.Peek())
	app.treeScroll.SetOffset(row + 1)
	if app.treeScroll.GetOffset() != row+1 {
		tt.Fatal("the tree can't scroll the cursor's row out of view, so this proves nothing")
	}
	s.render()
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Error("the summary shows for a row scrolled out of view")
	}
}

// underSummary returns a cell of the widget id that the summary covers.
func (s *screen) underSummary(tt *testing.T, id string) (int, int) {
	tt.Helper()
	bubble := s.summary(tt)
	target := s.renderer.WidgetByID(id)
	if target == nil {
		tt.Fatalf("%s is not on screen", id)
	}
	v := target.Visible
	for y := v.Y; y < v.Y+v.Height; y++ {
		for x := v.X; x < v.X+v.Width; x++ {
			if bubble.Contains(x, y) {
				return x, y
			}
		}
	}
	tt.Fatalf("the summary at %v doesn't cover %s at %v", bubble, id, v)
	return 0, 0
}

func TestSummaryLetsThePointerThrough(tt *testing.T) {
	// The first request in the tree, so its bubble reaches up to the tabs.
	app := appWithDescription("Current user", "Fetch the signed-in user's profile.", nil)
	app.openRequest(sampleRequest(tt, "Create user"))
	moveTreeCursor(tt, app, "Current user")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)

	// A click on a tab under the bubble reaches the tab, with no pointer
	// movement over the bubble first.
	body := tabID(requestTabsID, "body")
	x, y := s.underSummary(tt, body)
	entry := s.renderer.WidgetAt(x, y)
	clickable, ok := entry.EventWidget.(t.Clickable)
	if !ok || entry.ID != body {
		tt.Fatalf("a click under the summary reaches %q (%T), not the body tab", entry.ID, entry.EventWidget)
	}
	clickable.OnClick(t.MouseEvent{X: x, Y: y, Button: uv.MouseLeft, ClickCount: 1})
	if got := app.current().requestTab.Peek(); got != "body" {
		tt.Fatalf("clicking the body tab under the summary left the %s tab showing", got)
	}
	s.render()

	// The wheel scrolls the body the bubble covers, not the tree beside it.
	x, y = s.underSummary(tt, "req-body-text")
	scrollables := s.renderer.ScrollablesAt(x, y)
	if len(scrollables) == 0 || scrollables[0].State != app.current().bodyScroll {
		tt.Fatal("the wheel under the summary doesn't reach the request body")
	}

	// A click there focuses the body, which leaves the tree and so puts the
	// bubble away.
	focusable := s.renderer.FocusableAt(x, y)
	if focusable == nil || focusable.ID != "req-body-text" {
		tt.Fatalf("a click under the summary focuses %v, not the request body", focusable)
	}
	s.focusID(tt, focusable.ID)
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Error("the summary stayed after focus left the tree")
	}
}

func TestKeyboardSummarySurvivesAStillPointer(tt *testing.T) {
	app := testApp()
	dwells := holdTimers(tt, app)
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)
	bubble := s.summary(tt)

	// The pointer rests where the bubble appears: layout tells whatever is
	// beneath that the pointer is over it. Then the pointer moves within the
	// bubble, over another row. Neither puts the bubble away; only moving
	// focus, or resting the pointer on another row, does.
	p := &pointer{s: s}
	x, y := bubble.X+2, bubble.Y+1
	for _, source := range []t.HoverEventSource{t.HoverSourceLayout, t.HoverSourcePointer} {
		if entry := s.renderer.WidgetAt(x, y); entry == nil || entry.ID == summaryID {
			tt.Fatalf("the summary takes the pointer at (%d,%d)", x, y)
		}
		p.moveTo(x, y, source)
		s.render()
		s.summary(tt)
		y++
	}
	if len(*dwells) != 1 {
		tt.Fatalf("%d dwells started; want one, for the pointer moving onto a row", len(*dwells))
	}
}

// holdTimers stands in for app's timer, returning what it has been asked
// to run later, for the test to run when it likes. Only the summary's dwell
// uses the timer.
func holdTimers(tt *testing.T, app *App) *[]func() {
	tt.Helper()
	var held []func()
	app.after = func(d time.Duration, fn func()) {
		if d != summaryDwell {
			tt.Errorf("timer set for %v, want the summary dwell of %v", d, summaryDwell)
		}
		held = append(held, fn)
	}
	return &held
}

// runLast runs the last function held by holdTimers.
func runLast(tt *testing.T, held *[]func()) {
	tt.Helper()
	if len(*held) == 0 {
		tt.Fatal("nothing is waiting on the timer")
	}
	(*held)[len(*held)-1]()
}

// pointer moves the mouse over a screen the way the app tracks hover: the
// widget it was over hears it leave, then the one under it hears it enter.
type pointer struct {
	s    *screen
	id   string
	over t.Widget
}

func (p *pointer) moveTo(x, y int, source t.HoverEventSource) {
	id, widget := "", t.Widget(nil)
	if entry := p.s.renderer.WidgetAt(x, y); entry != nil {
		id, widget = entry.ID, entry.EventWidget
	}
	if id == p.id {
		return
	}
	if hoverable, ok := p.over.(t.Hoverable); ok {
		hoverable.OnHover(t.HoverEvent{Type: t.HoverLeave, Source: source, X: x, Y: y})
	}
	p.id, p.over = id, widget
	if hoverable, ok := widget.(t.Hoverable); ok {
		hoverable.OnHover(t.HoverEvent{Type: t.HoverEnter, Source: source, X: x, Y: y})
	}
}

// treeArea is the part of the screen the tree's viewport shows.
func (s *screen) treeArea() t.Rect { return s.renderer.WidgetByID(treeViewportID).Visible }

// label is the screen cell of the start of text in the tree.
func (s *screen) label(tt *testing.T, text string) (int, int) {
	tt.Helper()
	tree := s.treeArea()
	for y, line := range strings.Split(s.renderer.ScreenText(), "\n") {
		if y < tree.Y || y >= tree.Y+tree.Height {
			continue
		}
		row := string([]rune(line)[tree.X : tree.X+tree.Width])
		if i := strings.Index(row, text); i >= 0 {
			return tree.X + utf8.RuneCountInString(row[:i]), y
		}
	}
	tt.Fatalf("%q is not in the tree on screen", text)
	return 0, 0
}

// press presses a button (the left, unless given) at (x, y) as the app
// does: it focuses the focusable there, then tells the widget under the
// pointer, the widget that owns it (the tree, for a row) and, as a click,
// the widget again.
func (s *screen) press(tt *testing.T, x, y int, button ...uv.MouseButton) {
	tt.Helper()
	pressed := uv.MouseLeft
	if len(button) > 0 {
		pressed = button[0]
	}
	entry := s.renderer.WidgetAt(x, y)
	if entry == nil {
		tt.Fatalf("nothing at (%d,%d)", x, y)
	}
	if focusable := s.renderer.FocusableAt(x, y); focusable != nil {
		s.focus.FocusByID(focusable.ID)
		s.focused.Set(s.focus.Focused())
	}
	event := func(e *t.WidgetEntry) t.MouseEvent {
		return t.MouseEvent{X: x, Y: y, LocalX: x - e.Bounds.X, LocalY: y - e.Bounds.Y, Button: pressed, ClickCount: 1}
	}
	if down, ok := entry.EventWidget.(t.MouseDownHandler); ok {
		down.OnMouseDown(event(entry))
	}
	if owner := s.renderer.PointerOwnerAt(x, y); owner != nil && owner.ID != entry.ID {
		if down, ok := owner.EventWidget.(t.MouseDownHandler); ok {
			down.OnMouseDown(event(owner))
		}
	}
	if clickable, ok := entry.EventWidget.(t.Clickable); ok {
		clickable.OnClick(event(entry))
	}
	s.render()
}

func TestSummaryShowsWhenThePointerRestsOnARequest(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	dwells := holdTimers(tt, app)
	s := newScreen(app, snapW, snapH)
	p := &pointer{s: s}

	// The pointer rests on a request while the tree isn't focused, with the
	// cursor elsewhere. Only once it has rested for the dwell does the
	// summary show, for the hovered request, beside its row.
	x, y := s.label(tt, "List users")
	p.moveTo(x+2, y, t.HoverSourcePointer)
	s.render()
	if s.visible(summaryID) {
		tt.Fatal("the summary showed as soon as the pointer reached the row")
	}
	runLast(tt, dwells)
	s.render()
	bubble := s.summaryOf(tt, requestPath(tt, app, "List users"))
	if title := s.summaryTitle(tt); !strings.Contains(title, "List users") {
		tt.Fatalf("the summary is titled %q, not after the hovered request", title)
	}

	// Moving along the row, or over the bubble beside it (which the pointer
	// passes through to the row), keeps it.
	p.moveTo(bubble.X+4, y, t.HoverSourcePointer)
	s.render()
	s.summaryOf(tt, requestPath(tt, app, "List users"))

	// Leaving the row puts it away, and a dwell overtaken by the pointer
	// moving on brings nothing back.
	p.moveTo(x, y+1, t.HoverSourcePointer)
	s.render()
	if s.visible(summaryID) {
		tt.Fatal("the summary stayed after the pointer left its row")
	}
	p.moveTo(s.treeArea().X+s.treeArea().Width+10, y, t.HoverSourcePointer)
	runLast(tt, dwells)
	s.render()
	if s.visible(summaryID) {
		tt.Fatal("a dwell the pointer had moved on from showed a summary")
	}

	// Rows moving under a still pointer, as the tree scrolls, start no
	// dwell.
	started := len(*dwells)
	p.moveTo(x, y, t.HoverSourceLayout)
	if len(*dwells) != started {
		tt.Error("a row moving under the pointer started a dwell")
	}
}

func TestClickingARowDoesNotShowItsSummary(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	dwells := holdTimers(tt, app)
	s := newScreen(app, snapW, snapH)
	p := &pointer{s: s}

	// A hovered row's summary goes away when it is clicked, and the click,
	// which focuses the tree and moves its cursor, shows none.
	x, y := s.label(tt, "List users")
	p.moveTo(x, y, t.HoverSourcePointer)
	runLast(tt, dwells)
	s.render()
	s.summaryOf(tt, requestPath(tt, app, "List users"))
	s.press(tt, x, y)
	if s.focus.FocusedID() != treeID || !slices.Equal(app.tree.CursorPath.Peek(), requestPath(tt, app, "List users")) {
		tt.Fatal("the click didn't focus the tree and move its cursor to the row")
	}
	if s.visible(summaryID) {
		tt.Fatal("the summary shows after a click")
	}
	// The pointer staying on the row doesn't bring it back.
	p.moveTo(x+1, y, t.HoverSourcePointer)
	s.render()
	if s.visible(summaryID) {
		tt.Fatal("the summary came back with the pointer still on the clicked row")
	}

	// Moving through the tree with the keyboard shows the cursor's summary
	// again.
	pressOn(tt, app, treeID, "down")
	pressOn(tt, app, treeID, "up")
	s.render()
	s.summaryOf(tt, requestPath(tt, app, "List users"))

	// A press of any button puts it away, even one that moves the cursor
	// without opening the row. Focusing the tree again doesn't bring it
	// back; only the keyboard does.
	s.press(tt, x, y, uv.MouseRight)
	if s.visible(summaryID) {
		tt.Fatal("the summary shows after a right click")
	}
	x, y = s.label(tt, "Get user")
	s.press(tt, x, y, uv.MouseRight)
	if !slices.Equal(app.tree.CursorPath.Peek(), requestPath(tt, app, "Get user")) {
		tt.Fatal("a right click didn't move the tree cursor")
	}
	if s.visible(summaryID) {
		tt.Fatal("the summary shows after a right click moved the cursor")
	}
	s.focusID(tt, urlInputID)
	s.focusID(tt, treeID)
	if s.visible(summaryID) {
		tt.Fatal("refocusing the tree after a click showed the summary")
	}
}

func TestKeyboardTakesTheSummaryFromThePointer(tt *testing.T) {
	app := testApp()
	dwells := holdTimers(tt, app)
	moveTreeCursor(tt, app, "Get user")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)
	s.summary(tt)
	p := &pointer{s: s}

	// The pointer resting on another row takes the summary there, and when
	// the pointer leaves the tree, the cursor's summary doesn't come back.
	x, y := s.label(tt, "Login")
	p.moveTo(x, y, t.HoverSourcePointer)
	s.render()
	s.summary(tt)
	runLast(tt, dwells)
	s.render()
	s.summaryOf(tt, requestPath(tt, app, "Login"))
	p.moveTo(s.treeArea().X+s.treeArea().Width+10, y, t.HoverSourcePointer)
	s.render()
	if s.visible(summaryID) {
		tt.Fatal("the cursor's summary came back when the pointer left the tree")
	}

	// Moving the cursor by keyboard shows the cursor's summary, and a dwell
	// begun before it doesn't take the summary back.
	p.moveTo(x, y, t.HoverSourcePointer)
	pressOn(tt, app, treeID, "down")
	runLast(tt, dwells)
	s.render()
	s.summaryOf(tt, requestPath(tt, app, "List users"))
}

func TestWrapWords(tt *testing.T) {
	got := wrapWords("One two three four\n\nfive `six`", 10, 8)
	want := []string{"One two", "three four", "", "five `six`"}
	if !slices.Equal(got, want) {
		tt.Errorf("wrapWords = %q, want %q", got, want)
	}
	got = wrapWords("a b c d e f", 3, 2)
	want = []string{"a b", "c …"}
	if !slices.Equal(got, want) {
		tt.Errorf("wrapWords past the limit = %q, want %q", got, want)
	}
}
