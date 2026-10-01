package ui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/history"
	"github.com/darrenburns/posting/internal/model"
)

func TestReviewEscapeStartsNewTreeRange(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "Get user")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "escape")
	s.pressKey(tt, "shift+down")
	want := []string{"POST Create user", "PATCH Update user"}
	if got := selectedRows(app); !slices.Equal(got, want) {
		tt.Fatalf("new range after Escape = %q, want %q", got, want)
	}
}

func TestReviewHistorySelectionSurvivesResponse(tt *testing.T) {
	app, s := historyScreen(tt)
	s.pressKey(tt, "shift+down")
	app.nextHistoryID = 4
	app.openRequest(sampleRequest(tt, "Login"))
	updates := make(chan func(), 1)
	app.current().dispatch = func(fn func()) { updates <- fn }
	app.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) { return fixedResponse(), nil })
	app.send()
	select {
	case fn := <-updates:
		fn()
	case <-time.After(2 * time.Second):
		tt.Fatal("sender did not finish")
	}
	s.render()
	s.pressKey(tt, "backspace")
	if got := historyIDs(app); !slices.Equal(got, []int64{5, 3, 4}) {
		tt.Fatalf("history after deleting selection of original IDs 1,2 = %v, want [5 3 4]", got)
	}
}

func TestReviewLeftClearsTreeSelection(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "Get user")
	s := sidebarScreen(tt, app, treeID)
	s.pressKey(tt, "shift+down")
	s.pressKey(tt, "left")
	item, _ := app.tree.CursorNode()
	if item.Folder == nil {
		tt.Fatal("cursor not on parent folder")
	}
	s.pressKey(tt, "enter")
	if !app.tree.IsCollapsed(app.tree.CursorPath.Peek()) {
		tt.Fatalf("Enter on folder with cursor moved without shift opened requests instead of collapsing; selected %q", selectedRows(app))
	}
}

func TestReviewEditorKeysDoNotOperateOnTree(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "GET Get user", 2)
	s.focusID(tt, urlInputID)
	for _, kb := range s.focus.ActiveKeybinds() {
		if kb.Key == "d" && kb.Name == "Duplicate 3" {
			tt.Fatal("tree duplicate leaked into URL input")
		}
		if kb.Key == "backspace" && kb.Name == "Delete 3" {
			tt.Fatal("tree delete leaked into URL input")
		}
	}
	if len(store.deleted) != 0 || len(store.saved) != 0 {
		tt.Fatal("focus transition mutated store")
	}
}

func TestReviewCanceledDeleteAndDirtyOpen(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	app.openRequest(sampleRequest(tt, "Get user"))
	dirty := app.current()
	dirty.url.SetText("https://unsaved.test")
	dirty.urlEdited()
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "GET Get user", 2)
	s.pressKey(tt, "backspace")
	clickID(tt, app, "confirm-no")
	if len(store.deleted) != 0 || len(selectedRows(app)) != 3 {
		tt.Fatal("cancel deleted requests or discarded selection")
	}
	s.focusID(tt, treeID)
	s.pressKey(tt, "enter")
	if !dirty.dirty.Peek() || dirty.url.GetText() != "https://unsaved.test" {
		tt.Fatal("multi-open discarded dirty edits")
	}
	s.focusID(tt, treeID)
	s.pressKey(tt, "backspace")
	app.confirm.Peek().onYes()
	if dirty.file.Peek() != "" || !dirty.dirty.Peek() || dirty.url.GetText() != "https://unsaved.test" {
		tt.Fatal("delete discarded dirty edits")
	}
}

func TestReviewFilteredSelectionNeverDeletesHiddenRequests(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "GET Get user", 2)
	app.searchTree("create")
	s.render()
	s.pressKey(tt, "backspace")
	app.confirm.Peek().onYes()
	if !slices.Equal(store.deleted, []string{"users/create-user.posting.yaml"}) {
		tt.Fatalf("deleted hidden requests %v", store.deleted)
	}
}

func TestAdversarialGraphQLReloadSelection(tt *testing.T) {
	for _, field := range []string{"query", "variables"} {
		tt.Run(field, func(tt *testing.T) {
			app := testApp()
			req := model.GraphQLKind.New()
			req.File = "selected.posting.yaml"
			req.Payload = model.GraphQL{Query: "query Long { viewer { name email id } }", Variables: `{"name":"a sufficiently long value"}`}
			app.openRequest(req)
			s := app.current()
			editor := s.payloads[model.KindGraphQL].(*graphQLEditor)
			area, id := editor.query, graphQLQueryID
			if field == "variables" {
				area, id = editor.variables, graphQLVariablesID
				s.selectRequestTab("gql-variables")
			}
			area.CursorEnd()
			area.SetSelectionAnchor(len(area.Content.Peek()) - 3)
			area.CursorIndex.Set(len(area.Content.Peek()) - 1)
			req.Payload = model.GraphQL{Query: "{ x }", Variables: `{}`}
			app.replaceCollection(&model.Collection{Requests: []model.Request{req}})
			tt.Logf("after watcher reload: text=%q anchor=%d cursor=%d", area.GetText(), area.SelectionAnchor.Peek(), area.CursorIndex.Peek())
			defer func() {
				if failure := recover(); failure != nil {
					tt.Errorf("Backspace after a file reload panicked: %v", failure)
				}
			}()
			pressOn(tt, app, id, "backspace")
			want := "{ x }"
			if field == "variables" {
				want = "{}"
			}
			if area.GetText() != want {
				tt.Errorf("stale selection deleted replacement text: got %q, want %q", area.GetText(), want)
			}
		})
	}
}

func TestHTTPBodyReloadClearsSelection(tt *testing.T) {
	app := testApp()
	req := sampleRequest(tt, "Get user")
	req.Body = model.Body{Type: "raw", Raw: "a sufficiently long request body"}
	app.openRequest(req)
	s := app.current()
	s.selectRequestTab("body")
	s.body.CursorEnd()
	s.body.SetSelectionAnchor(20)
	req.Body.Raw = "short"
	app.replaceCollection(&model.Collection{Requests: []model.Request{req}})
	pressOn(tt, app, "req-body-text", "backspace")
	if got := s.body.GetText(); got != "short" {
		tt.Fatalf("reload left a stale body selection: %q", got)
	}
}

func TestTreeResetPreservesFilteredCollapsedView(tt *testing.T) {
	app := testApp()
	moveTreeCursor(tt, app, "Get user")
	folder := slices.Clone(app.tree.CursorPath.Peek()[:1])
	s := sidebarScreen(tt, app, treeID)
	app.tree.Collapse(folder)
	app.searchTree("user")
	s.render()
	selectFrom(tt, s, "GET Get user", 2)
	s.pressKey(tt, "escape")
	if !app.tree.IsCollapsed(folder) {
		tt.Fatal("reset expanded a collapsed folder")
	}
	if app.treeFilter.Query.Peek() != "user" {
		tt.Fatal("reset changed search")
	}
	shiftClickText(tt, app, "Update user", 0)
	if got, want := selectedRows(app), []string{"POST Create user", "PATCH Update user"}; !slices.Equal(got, want) {
		tt.Fatalf("new pointer range = %q, want %q", got, want)
	}
	s.render()
	s.pressKey(tt, "escape")
	s.pressKey(tt, "escape")
	s.pressKey(tt, "left")
	s.pressKey(tt, "left")
	if !app.tree.IsCollapsed(folder) {
		tt.Fatal("collapse after reset did not use stable folder identity")
	}
	s.pressKey(tt, "right")
	if app.tree.IsCollapsed(folder) {
		tt.Fatal("expand after reset failed")
	}
}

func TestHorizontalTreeMovesClearSelection(tt *testing.T) {
	for _, key := range []string{"left", "h", "right", "l"} {
		tt.Run(key, func(tt *testing.T) {
			app := testApp()
			s := sidebarScreen(tt, app, treeID)
			if key == "right" || key == "l" {
				selectFrom(tt, s, "GET Get user", 1)
				parent := slices.Clone(app.tree.CursorPath.Peek()[:1])
				app.tree.CursorPath.Set(parent)
				s.render()
			} else {
				selectFrom(tt, s, "GET Get user", 1)
			}
			s.pressKey(tt, key)
			if got := selectedRows(app); len(got) != 0 {
				tt.Fatalf("%s left selected rows %q", key, got)
			}
			s.pressKey(tt, "shift+down")
			if got := selectedRows(app); len(got) != 2 {
				tt.Fatalf("new range after %s = %q", key, got)
			}
		})
	}
}

func TestHistoryReplacementPreservesAnchorByID(tt *testing.T) {
	app, s := historyScreen(tt)
	s.pressKey(tt, "shift+down")
	entries := append([]model.HistoryEntry{{ID: 5}}, app.history.Peek()...)
	app.setHistory(entries)
	s.render()
	s.pressKey(tt, "shift+down")
	var ids []int64
	for _, entry := range app.historyList.SelectedItems() {
		ids = append(ids, entry.ID)
	}
	if !slices.Equal(ids, []int64{1, 2, 3}) {
		tt.Fatalf("range after prepend = %v", ids)
	}
	app.setHistory([]model.HistoryEntry{entries[0], entries[3], entries[4]})
	if app.historyList.HasAnchor() {
		tt.Fatal("removed anchor survived replacement")
	}
	if entry, ok := app.historyList.SelectedItem(); !ok || entry.ID != 3 {
		tt.Fatalf("cursor lost surviving ID 3: %+v", entry)
	}
	if got := app.historyList.SelectedItems(); len(got) != 1 || got[0].ID != 3 {
		tt.Fatalf("selection lost surviving ID 3: %+v", got)
	}
	app.setHistory(nil)
	if app.historyList.HasAnchor() || len(app.historyList.SelectedItems()) != 0 {
		tt.Fatal("empty history retained selection state")
	}
}

func TestHistoryTrimDropsRemovedSelectionAndAnchor(tt *testing.T) {
	app, screen := historyScreen(tt)
	entries := make([]model.HistoryEntry, history.MaxEntries)
	for i := range entries {
		entries[i] = model.HistoryEntry{ID: int64(i + 1)}
	}
	app.setHistory(entries)
	screen.render()
	screen.pressKey(tt, "end")
	screen.pressKey(tt, "shift+up")
	app.setHistory(append([]model.HistoryEntry{{ID: 101}}, entries...))
	screen.render()
	selected := app.historyList.SelectedItems()
	if len(selected) != 1 || selected[0].ID != 99 {
		tt.Fatalf("trim selected wrong entries: %+v", selected)
	}
	if app.historyList.HasAnchor() {
		tt.Fatal("trim retained removed anchor")
	}
	if cursor, ok := app.historyList.SelectedItem(); !ok || cursor.ID != 99 {
		tt.Fatalf("trim moved cursor from ID 99: %+v", cursor)
	}
	screen.pressKey(tt, "shift+up")
	selected = app.historyList.SelectedItems()
	if len(selected) != 2 || selected[0].ID != 98 || selected[1].ID != 99 {
		tt.Fatalf("range after trim = %+v", selected)
	}
}

func TestReviewerMouseBeforeRebuild(tt *testing.T) {
	app := testApp()
	s := sidebarScreen(tt, app, treeID)
	selectFrom(tt, s, "GET Get user", 2)
	// Escape action changes app state, but Terma does not synchronously render on key dispatch.
	app.clearTreeSelection()
	tt.Logf("screen=\n%s", s.renderer.ScreenText())
	lines := strings.Split(s.renderer.ScreenText(), "\n")
	for y, line := range lines {
		x := strings.Index(line, "Delete user")
		if x < 0 {
			continue
		}
		owner := s.renderer.PointerOwnerAt(x, y)
		if owner == nil {
			tt.Fatal("no owner")
		}
		down, ok := owner.EventWidget.(t.MouseDownHandler)
		if !ok {
			tt.Fatalf("owner %T lacks mouse", owner.EventWidget)
		}
		down.OnMouseDown(t.MouseEvent{X: x, Y: y, LocalX: x - owner.Bounds.X, LocalY: y - owner.Bounds.Y, Button: uv.MouseLeft, Mod: uv.ModShift, ClickCount: 1})
		got := selectedRows(app)
		tt.Logf("after shift click before render: selected=%q cursor=%v", got, app.tree.CursorPath.Peek())
		if len(got) != 3 || got[0] != "POST Create user" || got[2] != "DELETE Delete user" {
			tt.Errorf("wrong range from Escape cursor: %q", got)
		}
		s.render()
		tt.Logf("after render selected=%q", selectedRows(app))
		return
	}
	tt.Fatal("row absent")
}
