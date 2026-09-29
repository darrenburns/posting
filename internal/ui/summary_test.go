package ui

import (
	"slices"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/config"
	"github.com/darrenburns/posting/internal/model"
)

// moveTreeCursor puts the tree cursor on the request with the given name.
func moveTreeCursor(tt *testing.T, app *App, name string) {
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
	app.tree.CursorPath.Set(path)
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

// summary checks that the summary bubble is on screen, beside the tree and
// with its title (or, when it opens upwards, its last line) on the tree
// cursor's row, and returns its bounds.
func (s *screen) summary(tt *testing.T) t.Rect {
	tt.Helper()
	bubble, viewport := s.renderer.WidgetByID(summaryID), s.renderer.WidgetByID(treeViewportID)
	if bubble == nil || bubble.Visible.IsEmpty() {
		tt.Fatal("no summary beside the focused tree's cursor")
	}
	row, _ := s.app.treeRow(s.app.tree.CursorPath.Peek())
	want := viewport.Bounds.Y + row - s.app.treeScroll.GetOffset()
	b := bubble.Bounds
	if title, last := b.Y+1, b.Y+b.Height-2; title != want && last != want {
		tt.Fatalf("summary spans rows %d-%d, want its title or last line beside the cursor on row %d", title, last, want)
	}
	if left, edge := b.X, viewport.Bounds.X+viewport.Bounds.Width; left <= edge {
		tt.Fatalf("summary starts at column %d, over the tree, which ends at %d", left, edge)
	}
	return b
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
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)
	bubble := s.summary(tt)

	// The pointer rests where the bubble appears: layout tells whatever is
	// beneath that the pointer is over it. Then the pointer moves within the
	// bubble. Neither puts the bubble away; only moving focus does.
	x, y := bubble.X+2, bubble.Y+1
	for _, source := range []t.HoverEventSource{t.HoverSourceLayout, t.HoverSourcePointer} {
		entry := s.renderer.WidgetAt(x, y)
		if entry == nil || entry.ID == summaryID {
			tt.Fatalf("the summary takes the pointer at (%d,%d)", x, y)
		}
		if hoverable, ok := entry.EventWidget.(t.Hoverable); ok {
			hoverable.OnHover(t.HoverEvent{Type: t.HoverEnter, Source: source, X: x, Y: y})
		}
		s.render()
		s.summary(tt)
		y++
	}
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
