package ui

import "testing"

func pressTreeKey(app *App, key string) {
	for _, kb := range (collectionView{app: app}).Keybinds() {
		if kb.Key == key {
			kb.Action()
			return
		}
	}
}

func TestCollapseAllFoldsNestedFoldersAndKeepsCursorVisible(tt *testing.T) {
	app := testApp()
	before := len(app.visibleTreePaths())

	var nested []int
	for _, p := range app.visibleTreePaths() {
		if len(p) > 1 {
			nested = p
			break
		}
	}
	if nested == nil {
		tt.Fatal("the sample collection has no nested rows to test with")
	}
	app.tree.CursorPath.Set(nested)

	pressTreeKey(app, "E")
	for _, p := range app.visibleTreePaths() {
		if len(p) != 1 {
			tt.Fatalf("row %v still shown after collapse all, want only top-level rows", p)
		}
	}
	if got := app.tree.CursorPath.Peek(); len(got) != 1 || got[0] != nested[0] {
		tt.Errorf("cursor is %v after collapse all, want the top-level row [%d]", got, nested[0])
	}

	pressTreeKey(app, "e")
	if after := len(app.visibleTreePaths()); after != before {
		tt.Errorf("expand all shows %d rows, want the original %d", after, before)
	}
}

func treeHasKey(app *App, key string) bool {
	for _, kb := range (collectionView{app: app}).Keybinds() {
		if kb.Key == key {
			return true
		}
	}
	return false
}

func TestCollapseAllIsNotOfferedWhileSearching(tt *testing.T) {
	app := testApp()
	searchFor(app, "user")
	if treeHasKey(app, "e") || treeHasKey(app, "E") {
		tt.Error("expand/collapse all offered during a search, where it changes nothing on screen")
	}
	app.clearTreeSearch()
	if !treeHasKey(app, "e") || !treeHasKey(app, "E") {
		tt.Error("expand/collapse all missing once the search is cleared")
	}
}
