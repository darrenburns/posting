package ui

import (
	"slices"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/config"
	"github.com/darrenburns/posting/internal/model"
)

// focusOn focuses a widget when a snapshot renders, as a shortcut or click
// would in the running app.
type focusOn struct {
	id    string
	child t.Widget
}

func (f focusOn) Build(t.BuildContext) t.Widget {
	t.RequestFocus(f.id)
	return f.child
}

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
	t.AssertSnapshot(tt, focusOn{treeID, app}, snapW, snapH, "Collection focused: the highlighted request's summary floats beside its row")
}

func TestSnapshotRequestSummaryOpensUpwards(tt *testing.T) {
	app := appWithDescription("Delete user", longDescription, nil)
	moveTreeCursor(tt, app, "Delete user")
	t.AssertSnapshot(tt, focusOn{treeID, app}, snapW, 20, "Near the bottom of a short terminal, the summary opens upwards from the row")
}

func TestSnapshotRequestSummaryCollectionOnRight(tt *testing.T) {
	app := appWithDescription("List users", longDescription, func(s *config.Settings) {
		s.CollectionBrowser.Position = "right"
	})
	moveTreeCursor(tt, app, "List users")
	t.AssertSnapshot(tt, focusOn{treeID, app}, snapW, snapH, "Collection on the right: the summary floats to the left of the row")
}

func TestSummaryShowsBesideTheFocusedTreeCursor(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Fatal("the summary shows while the tree isn't focused")
	}
	s.focusID(tt, treeID)
	bubble, viewport := s.renderer.WidgetByID(summaryID), s.renderer.WidgetByID(treeViewportID)
	if bubble == nil {
		tt.Fatal("no summary beside the focused tree's cursor")
	}
	row, _ := app.treeRow(app.tree.CursorPath.Peek())
	if got, want := bubble.Bounds.Y+1, viewport.Bounds.Y+row; got != want {
		tt.Errorf("summary title on row %d, want it beside the cursor on row %d", got, want)
	}
	if left, edge := bubble.Bounds.X, viewport.Bounds.X+viewport.Bounds.Width; left <= edge {
		tt.Errorf("summary starts at column %d, over the tree, which ends at %d", left, edge)
	}

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

func TestSummaryStepsAsideForThePointer(tt *testing.T) {
	app := appWithDescription("List users", longDescription, nil)
	moveTreeCursor(tt, app, "List users")
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, treeID)
	bubble := s.renderer.WidgetByID(summaryID)
	if bubble == nil {
		tt.Fatal("no summary to test")
	}
	hover := func(x, y int) {
		entry := s.renderer.WidgetAt(x, y)
		if hoverable, ok := entry.EventWidget.(t.Hoverable); ok {
			hoverable.OnHover(t.HoverEvent{Type: t.HoverEnter, X: x, Y: y})
		}
		s.render()
	}
	// Appearing under a pointer that is standing still leaves it be.
	x, y := bubble.Bounds.X+2, bubble.Bounds.Y+1
	hover(x, y)
	hover(x, y)
	if s.renderer.WidgetByID(summaryID) == nil {
		tt.Fatal("the summary hid under a pointer that didn't move")
	}
	// Moving over it hides it, so the pointer reaches the workspace.
	hover(x, y+3)
	if s.renderer.WidgetByID(summaryID) != nil {
		tt.Fatal("the summary stayed under a moving pointer")
	}
	// Until the cursor moves.
	app.summary.hidden.Set(false)
	s.render()
	if s.renderer.WidgetByID(summaryID) == nil {
		tt.Error("the summary didn't come back")
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
