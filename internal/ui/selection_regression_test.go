package ui

import (
	"context"
	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
	"slices"
	"testing"
	"time"
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
