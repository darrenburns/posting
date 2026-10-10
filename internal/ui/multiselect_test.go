package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

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

// selectFrom puts the tree cursor on the row named from, as shownRows names
// it, and extends the selection down by n rows with shift+down.
func selectFrom(tt *testing.T, s *screen, from string, n int) {
	tt.Helper()
	for i, row := range shownRows(s.app) {
		if row == from {
			s.app.tree.CursorPath.Set(s.app.visibleTreePaths()[i])
			s.render()
			for range n {
				s.pressKey(tt, "shift+down")
			}
			return
		}
	}
	tt.Fatalf("no row %q in the tree", from)
}

func TestDeletingASelection(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "users/", 2)
	if !strings.Contains(s.renderer.ScreenText(), "Delete 2") {
		tt.Fatalf("the footer doesn't count the two selected requests:\n%s", s.renderer.ScreenText())
	}
	s.pressKey(tt, "backspace")
	if app.overlay.Peek() != "confirm" {
		tt.Fatal("deleting a selection should ask first")
	}
	c := app.confirm.Peek()
	if c.title != "Delete 2 requests?" || !strings.Contains(c.message, "Get user") || !strings.Contains(c.message, "List users") {
		tt.Fatalf("confirmation = %q: %q", c.title, c.message)
	}
	c.onYes()
	want := []string{"users/get-user.posting.yaml", "users/list-users.posting.yaml"}
	if !slices.Equal(store.deleted, want) {
		tt.Fatalf("deleted %q, want %q; the selected folder isn't a request", store.deleted, want)
	}
	for _, file := range want {
		if app.fileExists(file) {
			tt.Errorf("%s is still in the collection", file)
		}
	}
	if !app.fileExists("users/create-user.posting.yaml") {
		tt.Error("a request outside the selection was deleted")
	}
	if got := app.toast.Peek().message; got != "Deleted 2 requests" {
		tt.Errorf("toast = %q", got)
	}
	if got := selectedRows(app); len(got) != 0 {
		tt.Errorf("%q still selected after the delete", got)
	}
}

func TestDuplicatingASelection(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	tabs := len(app.sessions.Peek())
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "GET Get user", 2)
	s.pressKey(tt, "d")
	want := []string{"users/get-user-copy.posting.yaml", "users/list-users-copy.posting.yaml", "users/create-user-copy.posting.yaml"}
	if !slices.Equal(store.saved, want) {
		tt.Fatalf("saved %q, want %q", store.saved, want)
	}
	if got, want := selectedRows(app), []string{"GET Get user (copy)", "GET List users (copy)", "POST Create user (copy)"}; !slices.Equal(got, want) {
		tt.Errorf("selected %q after duplicating, want the copies %q", got, want)
	}
	if got := len(app.sessions.Peek()); got != tabs {
		tt.Errorf("duplicating a selection opened %d tabs", got-tabs)
	}
	if got := app.toast.Peek().message; got != "Duplicated 3 requests" {
		tt.Errorf("toast = %q", got)
	}
}

func TestSidebarOperationsWithoutASelectionActOnTheCursor(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	moveTreeCursor(tt, app, "List users")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "d")
	if got := app.current().file.Peek(); got != "users/list-users-copy.posting.yaml" {
		tt.Fatalf("d opened %q, want the copy of List users", got)
	}
	if got := app.toast.Peek().message; got != "Duplicated as users/list-users-copy.posting.yaml" {
		tt.Errorf("toast = %q", got)
	}
	s.focusID(tt, treeID)
	s.pressKey(tt, "backspace")
	if got := app.confirm.Peek().title; got != "Delete request?" {
		tt.Fatalf("backspace asked %q", got)
	}
	app.confirm.Peek().onYes()
	if !slices.Equal(store.deleted, []string{"users/list-users.posting.yaml"}) {
		tt.Fatalf("deleted %q", store.deleted)
	}
	if got := app.toast.Peek().message; got != "Deleted users/list-users.posting.yaml" {
		tt.Errorf("toast = %q", got)
	}
}

func TestEnterOpensEachSelectedRequestInATab(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "users/", 3)
	s.pressKey(tt, "enter")
	assertTabs(tt, app, "List users, Get user, Create user")
	assertActive(tt, app, "Create user")

	s.pressKey(tt, "down")
	s.pressKey(tt, "enter")
	assertTabs(tt, app, "List users, Get user, Create user, /Update user/")
}

// historyScreen is the sample app showing four history entries, with IDs 1
// to 4 from the top, and focus in the history list.
func historyScreen(tt *testing.T) (*App, *screen) {
	tt.Helper()
	app := testApp()
	var entries []model.HistoryEntry
	for i, name := range []string{"List users", "Get user", "Create user", "Login"} {
		entries = append(entries, model.HistoryEntry{ID: int64(i + 1), Request: sampleRequest(tt, name), Response: fixedResponse(), SentAt: time.Now()})
	}
	app.setHistory(entries)
	app.sidebarTab.Set("history")
	return app, sidebarScreen(tt, app, historyID)
}

func historyIDs(app *App) []int64 {
	var ids []int64
	for _, e := range app.history.Peek() {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestDeletingSelectedHistory(tt *testing.T) {
	app, s := historyScreen(tt)
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "shift+down")
	if !strings.Contains(s.renderer.ScreenText(), "Delete 3 entries") {
		tt.Fatalf("the footer doesn't count the three selected entries:\n%s", s.renderer.ScreenText())
	}
	s.pressKey(tt, "backspace")
	if got := historyIDs(app); !slices.Equal(got, []int64{4}) {
		tt.Fatalf("history after deleting the first three = %v, want [4]", got)
	}
	if got := app.historyList.SelectedItems(); len(got) != 0 {
		tt.Errorf("%d entries still selected after the delete", len(got))
	}

	s.pressKey(tt, "backspace")
	if got := historyIDs(app); len(got) != 0 {
		tt.Fatalf("backspace without a selection left %v", got)
	}
}

func TestEscapeClearsTheHistorySelection(tt *testing.T) {
	app, s := historyScreen(tt)
	s.pressKey(tt, "shift+down")
	if len(app.historyList.SelectedItems()) != 2 {
		tt.Fatal("shift+down didn't select two entries")
	}
	s.pressKey(tt, "escape")
	if got := app.historyList.SelectedItems(); len(got) != 0 {
		tt.Fatalf("escape left %d entries selected", len(got))
	}
	s.pressKey(tt, "shift+down")
	var ids []int64
	for _, e := range app.historyList.SelectedItems() {
		ids = append(ids, e.ID)
	}
	if !slices.Equal(ids, []int64{2, 3}) {
		tt.Fatalf("shift+down after escape selected %v, want a new range from the cursor, [2 3]", ids)
	}
}
