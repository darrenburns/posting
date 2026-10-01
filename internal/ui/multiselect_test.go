package ui

import (
	"slices"
	"testing"

	t "github.com/darrenburns/terma"
)

// sidebarScreen is the sample app on screen with focus in the widget id.
func sidebarScreen(tt *testing.T, app *App, id string) *screen {
	tt.Helper()
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, id)
	return s
}

// pressKey runs the first active keybind for key, checking the focused widget
// and then its ancestors as the focus manager does, and renders the result.
func (s *screen) pressKey(tt *testing.T, key string) {
	tt.Helper()
	for _, kb := range s.focus.ActiveKeybinds() {
		if kb.Key == key {
			kb.Action()
			s.render()
			return
		}
	}
	tt.Fatalf("nothing on screen handles %q", key)
}

// selectedRows lists the selected rows of the request tree, as shownRows
// names them.
func selectedRows(app *App) []string {
	var rows []string
	shown := shownRows(app)
	for i, p := range app.visibleTreePaths() {
		if app.tree.IsSelected(p) {
			rows = append(rows, shown[i])
		}
	}
	return rows
}

func TestShiftDownSelectsARangeOfRequests(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "Get user")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "shift+down")
	want := []string{"GET Get user", "GET List users", "POST Create user"}
	if got := selectedRows(app); !slices.Equal(got, want) {
		tt.Fatalf("shift+down twice from Get user selected %q, want %q", got, want)
	}
	s.pressKey(tt, "down")
	if got := selectedRows(app); len(got) != 0 {
		tt.Fatalf("a plain move left %q selected", got)
	}
}

func TestEscapeClearsTheSelectionBeforeTheSearch(tt *testing.T) {
	app := testApp()
	searchFor(app, "user")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "shift+down")
	if len(selectedRows(app)) == 0 {
		tt.Fatal("shift+down in the results selected nothing")
	}
	s.pressKey(tt, "escape")
	if got := selectedRows(app); len(got) != 0 {
		tt.Fatalf("escape left %q selected", got)
	}
	if got := app.treeFilter.Query.Peek(); got != "user" {
		tt.Fatalf("the first escape cleared the search too, query = %q", got)
	}
	s.pressKey(tt, "escape")
	if got := app.treeFilter.Query.Peek(); got != "" {
		tt.Fatalf("the second escape left the search %q", got)
	}
}

func TestSnapshotTreeMultiSelect(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "Get user")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "shift+down")
	t.RequestFocus(treeID)
	t.AssertSnapshot(tt, app, snapW, snapH, "Three requests in the users folder selected with shift+down, the cursor on the last of them")
}
