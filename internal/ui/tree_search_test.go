package ui

import (
	"slices"
	"testing"

	t "github.com/darrenburns/terma"
)

// searchFor types query into the collection's search box.
func searchFor(app *App, query string) {
	app.treeSearch.SetText(query)
	app.searchTree(query)
}

// focusAfterRender renders app and returns the ID of the widget that has
// focus, after any focus request.
func focusAfterRender(app *App) string {
	probe := &responseFocusProbe{app: app}
	t.RenderToBuffer(probe, snapW, snapH)
	return probe.focused
}

// shownRows lists the rows of the request tree: folder names, and requests
// as "METHOD name".
func shownRows(app *App) []string {
	var rows []string
	for _, p := range app.visibleTreePaths() {
		node, _ := app.tree.NodeAtPath(p)
		if r := node.Data.Request; r != nil {
			rows = append(rows, string(r.Method)+" "+r.Name)
		} else {
			rows = append(rows, node.Data.Folder.Name+"/")
		}
	}
	return rows
}

// cursorRow is the tree row under the cursor, as shownRows names it.
func cursorRow(app *App) string {
	cursor := app.tree.CursorPath.Peek()
	for i, p := range app.visibleTreePaths() {
		if slices.Equal(p, cursor) {
			return shownRows(app)[i]
		}
	}
	return ""
}

func TestTreeSearchMatches(tt *testing.T) {
	for _, tc := range []struct {
		query string
		want  []string
	}{
		// The method, whole or begun, finds requests; the folders they're
		// in stay above them.
		{"POST", []string{"auth/", "POST Login", "users/", "POST Create user"}},
		{"pa", []string{"users/", "PATCH Update user"}},
		// Terms match in any order, each in the name, the method or a
		// folder.
		{"post users", []string{"users/", "POST Create user"}},
		{"user post", []string{"users/", "POST Create user"}},
		{"delete user", []string{"users/", "DELETE Delete user"}},
		{"CHECK", []string{"GET Health check"}},
		// A folder's name finds everything in it.
		{"auth", []string{"auth/", "GET Current user", "POST Login"}},
		{"  ", []string{"auth/", "GET Current user", "POST Login", "users/", "GET Get user", "GET List users", "POST Create user", "PATCH Update user", "DELETE Delete user", "GET Health check", "OPTIONS CORS preflight"}},
		{"nothing like this", nil},
	} {
		app := testApp()
		searchFor(app, tc.query)
		if got := shownRows(app); !slices.Equal(got, tc.want) {
			tt.Errorf("searching %q shows %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestTreeSearchKeyboardFlow(tt *testing.T) {
	app := testApp()
	app.focusTreeSearch()
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Fatalf("the search keybinding focused %q, want the search box", got)
	}

	// Typing puts the cursor on the first matching request, and down moves
	// into the results.
	searchFor(app, "post")
	if got := cursorRow(app); got != "POST Login" {
		tt.Errorf("after searching, the cursor is on %q, want the first match", got)
	}
	pressOn(tt, app, treeSearchID, "down")
	if got := focusAfterRender(app); got != treeID {
		tt.Fatalf("down from the search box focused %q, want the tree", got)
	}

	// Up walks the results, then leaves the top row for the search box.
	pressOn(tt, app, treeID, "up")
	if got := cursorRow(app); got != "auth/" {
		tt.Errorf("up from the first match moved the cursor to %q, want its folder", got)
	}
	pressOn(tt, app, treeID, "up")
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Fatalf("up from the top of the tree focused %q, want the search box", got)
	}

	// Enter goes back to the results, which open requests as usual.
	pressOn(tt, app, treeSearchID, "enter")
	if got := focusAfterRender(app); got != treeID {
		tt.Fatalf("enter in the search box focused %q, want the tree", got)
	}
	for range 3 {
		pressOn(tt, app, treeID, "down")
	}
	if got := cursorRow(app); got != "POST Create user" {
		tt.Fatalf("down through the results reached %q, want the last match", got)
	}
	pressOn(tt, app, treeID, "enter")
	if got := app.current().file.Peek(); got != "users/create-user.posting.yaml" {
		tt.Errorf("enter on a search result opened %q, want Create user", got)
	}

	// Escape in the tree clears the search, and the request the cursor was
	// on stays in view, in its folder, now expanded.
	app.tree.Collapse([]int{1})
	pressOn(tt, app, treeID, "escape")
	if got := app.treeSearch.GetText(); got != "" {
		tt.Errorf("escape left %q in the search box", got)
	}
	if got := cursorRow(app); got != "POST Create user" {
		tt.Errorf("after clearing the search the cursor is on %q, want the request it was on", got)
	}
	if got := len(shownRows(app)); got != 11 {
		tt.Errorf("after clearing the search the tree shows %d rows, want all 11", got)
	}
	if got := focusAfterRender(app); got != treeID {
		tt.Errorf("escape in the tree focused %q, want the tree", got)
	}

	// Without a search, up from the top row still reaches the box, and
	// escape there returns to the tree.
	app.tree.CursorPath.Set([]int{0})
	pressOn(tt, app, treeID, "up")
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Fatalf("up from the top of the tree focused %q, want the search box", got)
	}
	searchFor(app, "health")
	pressOn(tt, app, treeSearchID, "escape")
	if got := app.treeFilter.PeekQuery(); got != "" {
		tt.Errorf("escape in the search box left the tree filtered by %q", got)
	}
	if got := focusAfterRender(app); got != treeID {
		tt.Errorf("escape in the search box focused %q, want the tree", got)
	}
}

func TestTreeSearchRowsForTheSummary(tt *testing.T) {
	// The summary bubble finds its row among the results.
	app := testApp()
	searchFor(app, "post")
	moveTreeCursor(tt, app, "Create user")
	if row, ok := app.treeRow(app.tree.CursorPath.Peek()); !ok || row != 3 {
		tt.Errorf("Create user is on row %d (%v) of the results, want 3", row, ok)
	}
}

func TestTreeSearchWithNoResults(tt *testing.T) {
	app := testApp()
	app.focusTreeSearch()
	searchFor(app, "nothing like this")
	pressOn(tt, app, treeSearchID, "down")
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Errorf("down with no results focused %q, want to stay in the search box", got)
	}
}

func TestTreeSearchKeybindingShowsTheCollection(tt *testing.T) {
	app := testApp()
	app.sidebarVisible.Set(false)
	app.sidebarTab.Set("history")
	for _, kb := range app.Keybinds() {
		if kb.Key == "ctrl+g" {
			kb.Action()
		}
	}
	if !app.sidebarVisible.Peek() || app.sidebarTab.Peek() != "requests" {
		tt.Fatal("the search keybinding didn't bring up the collection")
	}
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Errorf("the search keybinding focused %q, want the search box", got)
	}
	// "/" in the tree focuses the box too.
	for _, kb := range (collectionView{app: app}).Keybinds() {
		if kb.Key == "/" {
			kb.Action()
		}
	}
	if got := focusAfterRender(app); got != treeSearchID {
		tt.Errorf("/ in the tree focused %q, want the search box", got)
	}
}

func TestSnapshotTreeSearch(tt *testing.T) {
	app := testApp()
	searchFor(app, "post user")
	t.RequestFocus(treeSearchID)
	t.AssertSnapshotNamed(tt, "TreeSearch_typing", app, snapW, snapH, "Searching the collection for \"post user\": POST requests in or named after users, with the matches underlined")

	app = testApp()
	searchFor(app, "user")
	t.RequestFocus(treeID)
	t.AssertSnapshotNamed(tt, "TreeSearch_results", app, snapW, snapH, "Results for \"user\" focused: the cursor on the first match, everything in the users folder shown")

	app = testApp()
	searchFor(app, "nothing like this")
	t.RequestFocus(treeSearchID)
	t.AssertSnapshotNamed(tt, "TreeSearch_empty", app, snapW, snapH, "A search that matches nothing explains how to clear it")
}
