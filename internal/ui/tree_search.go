package ui

import (
	"slices"
	"strings"

	t "github.com/darrenburns/terma"
)

// Searching the collection happens in the tree itself. The box at the top of
// the Requests tab narrows the tree to the requests that match, keeping the
// folders they're in, through Terma's tree filtering (Tree.Filter and
// Tree.MatchNode): typing filters live, down or enter moves into the
// results, and escape clears the search.

const treeSearchID = "side-tree-search"

// treeQuery is a search of the collection split into lower-case terms. An
// item matches when every term, in any order, is found in its name, starts
// its method or badge (so "post" and "po" find POST requests, and "gql"
// GraphQL ones), or is found in the folders it's in (so "users" finds
// everything in the users folder). What matched is always on screen: in the
// row itself, or in a folder above it.
type treeQuery []string

func parseTreeQuery(s string) treeQuery { return strings.Fields(strings.ToLower(s)) }

func (q treeQuery) matches(i treeItem) bool {
	name, folders := "", i.folderPath()
	if i.Request != nil {
		name = i.Request.DisplayName()
	}
	for _, term := range q {
		if !containsFold(name, term) && !containsFold(folders, term) && !methodHasPrefix(i, term) {
			return false
		}
	}
	return true
}

func containsFold(text, term string) bool {
	return strings.Contains(strings.ToLower(text), term)
}

func methodHasPrefix(i treeItem, term string) bool {
	if i.Request == nil {
		return false
	}
	r := *i.Request
	return strings.HasPrefix(strings.ToLower(requestLabel(r)), term) || strings.HasPrefix(strings.ToLower(r.Badge()), term)
}

// matchTreeItem is the tree's MatchNode. The rows draw their own highlights
// (see treeQuery.spans), so it only decides what's shown.
func matchTreeItem(i treeItem, query string, _ t.FilterOptions) t.MatchResult {
	return t.MatchResult{Matched: parseTreeQuery(query).matches(i)}
}

// spans draws text in style, with every occurrence of the query's terms
// marked in highlight.
func (q treeQuery) spans(text string, style, highlight t.SpanStyle) []t.Span {
	var ranges []t.MatchRange
	for _, term := range q {
		ranges = append(ranges, t.MatchString(text, term, t.FilterOptions{}).Ranges...)
	}
	return markRanges(text, ranges, style, highlight)
}

// methodSpans draws a request's method label, marking the start of it when a
// term matched the method.
func (q treeQuery) methodSpans(label string, i treeItem, style, highlight t.SpanStyle) []t.Span {
	n := 0
	for _, term := range q {
		if methodHasPrefix(i, term) {
			n = max(n, min(len(term), len(strings.TrimRight(label, " "))))
		}
	}
	return markRanges(label, []t.MatchRange{{Start: 0, End: n}}, style, highlight)
}

// markRanges splits text into spans in style, with the ranges also taking
// highlight's underline and, where it has one, its background.
func markRanges(text string, ranges []t.MatchRange, style, highlight t.SpanStyle) []t.Span {
	marked := style
	marked.Underline, marked.UnderlineColor = highlight.Underline, highlight.UnderlineColor
	if highlight.Background.IsSet() {
		marked.Background = highlight.Background
	}
	spans := t.HighlightSpans(text, ranges, marked)
	for i := range spans {
		// HighlightSpans leaves the text between matches unstyled.
		if spans[i].Style == (t.SpanStyle{}) {
			spans[i].Style = style
		}
	}
	return spans
}

// visibleTreePaths lists the rows of the request tree from the top, as the
// Tree lays them out: while searching, the matches and the folders they're
// in, whether or not those are collapsed; otherwise the expanded tree. It
// doesn't subscribe to anything.
func (a *App) visibleTreePaths() [][]int {
	q := parseTreeQuery(a.treeFilter.Query.Peek())
	var paths [][]int
	var walk func(nodes []t.TreeNode[treeItem], prefix []int) bool
	walk = func(nodes []t.TreeNode[treeItem], prefix []int) bool {
		shown := false
		for i, node := range nodes {
			p := append(slices.Clip(prefix), i)
			at := len(paths)
			paths = append(paths, p)
			if len(q) == 0 {
				if len(node.Children) > 0 && !a.tree.IsCollapsed(p) {
					walk(node.Children, p)
				}
				shown = true
				continue
			}
			if walk(node.Children, p) || q.matches(node.Data) {
				shown = true
			} else {
				paths = paths[:at]
			}
		}
		return shown
	}
	walk(a.tree.Nodes.Peek(), nil)
	return paths
}

// firstTreeMatch is the first request that matches the search, or failing
// that the first folder, for the cursor to start on.
func (a *App) firstTreeMatch() ([]int, bool) {
	q := parseTreeQuery(a.treeFilter.Query.Peek())
	var folder []int
	for _, p := range a.visibleTreePaths() {
		node, ok := a.tree.NodeAtPath(p)
		if !ok || !q.matches(node.Data) {
			continue
		}
		if node.Data.Request != nil {
			return p, true
		}
		if folder == nil {
			folder = p
		}
	}
	return folder, folder != nil
}

// focusTreeSearch brings the collection into view and focuses its search
// box, selecting what's there so typing starts a new search.
func (a *App) focusTreeSearch() {
	a.sidebarVisible.Set(true)
	a.sidebarTab.Set("requests")
	if len(a.tree.Nodes.Peek()) == 0 {
		t.RequestFocus(sidebarTabsID)
		return
	}
	a.treeSearch.SelectAll()
	t.RequestFocus(treeSearchID)
}

// searchTree filters the tree as the search changes, putting the cursor on
// the first match so down or enter lands on it. The filter holds the
// normalised terms, so a search of only spaces is no search at all.
func (a *App) searchTree(text string) {
	query := strings.Join(parseTreeQuery(text), " ")
	a.treeFilter.Query.Set(query)
	if query == "" {
		a.revealTreeCursor()
		return
	}
	if p, ok := a.firstTreeMatch(); ok {
		a.tree.CursorPath.Set(p)
	}
}

// clearTreeSearch drops the search, leaving the cursor where it was in the
// results, and returns to the tree.
func (a *App) clearTreeSearch() {
	a.treeSearch.SetText("")
	a.searchTree("")
	t.RequestFocus(treeID)
}

// revealTreeCursor expands the folders around the cursor, so the row it was
// on in the search results is still in view once the search is gone.
func (a *App) revealTreeCursor() {
	cursor := a.tree.CursorPath.Peek()
	for n := 1; n < len(cursor); n++ {
		a.tree.Expand(cursor[:n])
	}
}

// enterTreeResults moves from the search box into the tree, if it shows
// anything.
func (a *App) enterTreeResults() {
	if len(a.visibleTreePaths()) > 0 {
		t.RequestFocus(treeID)
	}
}

// treeCursorAtTop reports whether the cursor is on the tree's first row.
func (a *App) treeCursorAtTop() bool {
	paths := a.visibleTreePaths()
	return len(paths) == 0 || slices.Equal(paths[0], a.tree.CursorPath.Peek())
}

// requestsPane is the Requests tab: the search box above the tree. An empty
// collection has nothing to search, so it's just the tree's empty state.
type requestsPane struct {
	fillParent
	app *App
}

func (p requestsPane) Build(ctx t.BuildContext) t.Widget {
	a := p.app
	if len(a.tree.Nodes.Get()) == 0 {
		return collectionView{app: a}
	}
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: a.gap(),
		Children: []t.Widget{
			input{
				ID:          treeSearchID,
				State:       a.treeSearch,
				Placeholder: "Search requests…",
				OnChange:    a.searchTree,
				OnSubmit:    func(string) { a.enterTreeResults() },
				Keybinds: []t.Keybind{
					{Key: "down", Name: "Results", Action: a.enterTreeResults, Hidden: true},
					{Key: "up", Name: "Out", Action: func() { t.RequestFocus(sidebarTabsID) }, Hidden: true},
					{Key: "escape", Name: "Clear", Action: a.clearTreeSearch},
				},
			},
			collectionView{app: a},
		},
	}
}

// noTreeResults stands in for the tree when nothing matches the search. It
// sits just under the search box, where the results would be.
type noTreeResults struct{ fillParent }

func (noTreeResults) Build(ctx t.BuildContext) t.Widget {
	text := t.ParseMarkupToText("[b]No matching requests[/]\n[$TextMuted]Press [b]esc[/] to clear the search[/]", ctx.Theme())
	text.Wrap = t.WrapSoft
	text.Style.Width = t.Flex(1)
	return t.Column{
		Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: inset},
		Children: []t.Widget{text},
	}
}

// collectionTree is the request tree with the keys that tie it to its search
// box: up from the first row goes back to the box, and escape clears the
// search.
type collectionTree struct {
	t.Tree[treeItem]
	app *App
}

func (c collectionTree) Keybinds() []t.Keybind {
	a := c.app
	var binds []t.Keybind
	if a.treeCursorAtTop() {
		binds = append(binds, t.Keybind{Key: "up", Name: "Search", Action: func() { t.RequestFocus(treeSearchID) }, Hidden: true})
	}
	if a.treeFilter.PeekQuery() != "" {
		binds = append(binds, t.Keybind{Key: "escape", Name: "Clear search", Action: a.clearTreeSearch})
	}
	return append(binds, c.Tree.Keybinds()...)
}
