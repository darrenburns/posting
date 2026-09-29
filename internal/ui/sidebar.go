package ui

import (
	"fmt"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const (
	sidebarSplitID = "sidebar-split"
	sidebarTabsID  = "side-tabs"
	treeID         = "side-tree"
	historyID      = "side-history"
)

// treeItem is a node in the collection tree: either a folder or a request.
type treeItem struct {
	Folder  *model.Collection
	Request *model.Request
}

func (i treeItem) key() string {
	if i.Request != nil {
		return "r:" + i.Request.File
	}
	return "f:" + i.Folder.Path
}

// folderPath is the folder that new requests created from this node go into.
func (i treeItem) folderPath() string {
	if i.Folder != nil {
		return i.Folder.Path
	}
	folder, _ := splitFile(i.Request.File)
	return folder
}

func buildTree(c *model.Collection) []t.TreeNode[treeItem] {
	nodes := make([]t.TreeNode[treeItem], 0, len(c.Children)+len(c.Requests))
	for _, child := range c.Children {
		nodes = append(nodes, t.TreeNode[treeItem]{Data: treeItem{Folder: child}, Children: buildTree(child)})
	}
	for i := range c.Requests {
		nodes = append(nodes, t.TreeNode[treeItem]{Data: treeItem{Request: &c.Requests[i]}})
	}
	return nodes
}

// cursorFolder is the folder under the tree cursor, for new requests.
func (a *App) cursorFolder() string {
	if item, ok := a.tree.CursorNode(); ok {
		return item.folderPath()
	}
	return ""
}

// sidebar is the collection browser.
type sidebar struct{ app *App }

func (sb sidebar) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), t.Flex(1) }

// Build lays the sidebar straight onto the app background, with no card
// around it; the split pane's divider separates it from the workspace.
func (sb sidebar) Build(ctx t.BuildContext) t.Widget {
	a := sb.app
	historyCount := len(a.history.Get())
	title := "[b $TextMuted]" + escapeMarkup(a.collection.Get().Name) + "[/]"
	if focusWithin(ctx, "side-") {
		title = "[b $Text]" + escapeMarkup(a.collection.Get().Name) + "[/]"
	}
	heading := t.ParseMarkupToText(title, ctx.Theme())
	heading.Style.Width = t.Flex(1)
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: t.EdgeInsets{Left: 2, Right: 1}},
		Spacing: a.gap(),
		Children: []t.Widget{
			heading,
			tabStrip{ID: sidebarTabsID, Active: a.sidebarTab, Down: a.focusBelowSidebarTabs, Tabs: []tabItem{
				{Key: "requests", Label: a.icons.requests + "Requests"},
				{Key: "history", Label: a.icons.history + "History", Badge: countBadge(historyCount)},
			}},
			t.Switcher{
				Active: a.sidebarTab.Get(),
				Style:  t.Style{Width: t.Flex(1), Height: t.Flex(1)},
				Children: map[string]t.Widget{
					"requests": requestsPane{app: a},
					"history":  historyView{app: a},
				},
			},
		},
	}
}

// collectionView is the request tree, with a summary of the highlighted
// request floating beside it (see requestSummary).
type collectionView struct {
	fillParent
	app *App
}

func (c collectionView) Keybinds() []t.Keybind {
	a := c.app
	return []t.Keybind{
		{Key: "/", Name: "Search", Action: a.focusTreeSearch},
		{Key: "d", Name: "Duplicate", Action: a.duplicateAtCursor},
		{Key: "backspace", Name: "Delete", Action: a.confirmDeleteAtCursor},
		{Key: "delete", Name: "Delete", Action: a.confirmDeleteAtCursor, Hidden: true},
	}
}

func (c collectionView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := c.app
	if len(a.tree.Nodes.Get()) == 0 {
		return emptyState{Title: "Collection is empty", Lines: []string{"Press [b]ctrl+s[/] to save the current request"}}
	}
	openFiles := map[string]bool{}
	for _, s := range a.sessions.Get() {
		if f := s.file.Get(); f != "" {
			openFiles[f] = true
		}
	}
	focused := isFocusedID(ctx, treeID)
	activeFile := ""
	if s := a.session(); s != nil {
		activeFile = s.file.Get()
	}
	query := parseTreeQuery(a.treeFilter.Query.Get())
	if len(query) > 0 && len(a.visibleTreePaths()) == 0 {
		return noTreeResults{}
	}
	// The tree fills the panel on its own: the summary of the request under
	// the cursor floats beside it, so the tree never resizes as it moves.
	return t.Stack{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{
			t.Scrollable{
				ID:    treeViewportID,
				State: a.treeScroll,
				Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
				Child: collectionTree{app: a, Tree: t.Tree[treeItem]{
					ID:             treeID,
					State:          a.tree,
					ScrollState:    a.treeScroll,
					Filter:         a.treeFilter,
					MatchNode:      matchTreeItem,
					NodeID:         func(i treeItem) string { return i.key() },
					HasChildren:    func(i treeItem) bool { return i.Folder != nil },
					ShowGuideLines: t.BoolPtr(false),
					// As in Posting 2, a click opens a request or folder.
					ActivateOnClick: true,
					OnSelect: func(i treeItem, _ []treeItem) {
						if i.Request != nil {
							a.openRequest(*i.Request)
						} else if len(query) == 0 {
							// Search results show every folder with a
							// match open, whatever its collapsed state.
							a.tree.Toggle(a.tree.CursorPath.Peek())
						}
					},
					RenderNode: func(i treeItem, node t.TreeNodeContext) t.Widget {
						return renderTreeNode(theme, a.icons, i, node, focused, openFiles, activeFile, query)
					},
					Style: t.Style{Width: t.Flex(1)},
				}},
			},
			requestSummary{app: a},
		},
	}
}

// renderTreeNode draws one row. The cursor is only emphasised while the tree
// has focus; otherwise it is a quiet highlight so it doesn't compete with the
// focused widget. Letters that match the search are highlighted.
func renderTreeNode(theme t.ThemeData, icons iconSet, i treeItem, node t.TreeNodeContext, focused bool, openFiles map[string]bool, activeFile string, query treeQuery) t.Widget {
	var bg t.Color
	cursor := node.Active && focused
	switch {
	case cursor:
		bg = theme.ActiveCursor
	case node.Active:
		bg = theme.Surface
	}
	highlight := t.MatchHighlightStyle(theme)
	if cursor {
		highlight.Background = t.Color{}
	}
	if i.Folder != nil {
		fg := theme.TextMuted
		if cursor {
			fg = theme.SelectionText
		}
		icon, suffix := icons.folder, ""
		if node.Expanded {
			icon = icons.folderOpen
		}
		if icon == "" {
			suffix = "/"
		}
		style := t.SpanStyle{Foreground: fg, Background: bg, Bold: true}
		spans := append([]t.Span{{Text: icon, Style: style}}, query.spans(i.Folder.Name, style, highlight)...)
		spans = append(spans, t.Span{Text: suffix, Style: style})
		return t.Text{Spans: spans, Style: t.Style{Width: t.Flex(1)}}
	}
	// The method lines up with folder names at the same depth. The request
	// in the visible tab is bold; others open in tabs get a dot after them.
	r := i.Request
	nameStyle := t.SpanStyle{Foreground: theme.Text, Background: bg}
	methodStyle := t.SpanStyle{Foreground: methodColor(theme, r.Method), Background: bg, Bold: true}
	markStyle := t.SpanStyle{Foreground: theme.TextMuted, Background: bg}
	mark := ""
	switch {
	case r.File == activeFile && activeFile != "":
		nameStyle.Bold = true
		nameStyle.Foreground = theme.PrimaryText
	case openFiles[r.File]:
		mark = " •"
	}
	if cursor {
		nameStyle.Foreground = theme.SelectionText
		methodStyle.Foreground = theme.SelectionText
		markStyle.Foreground = theme.SelectionText
	}
	spans := query.methodSpans(padRight(r.Method.Short(), 3)+" ", i, methodStyle, highlight)
	spans = append(spans, query.spans(r.DisplayName(), nameStyle, highlight)...)
	spans = append(spans, t.Span{Text: mark, Style: markStyle})
	return t.Text{Spans: spans, Style: t.Style{Width: t.Flex(1)}}
}

// historyView lists previously sent requests, newest first.
type historyView struct {
	fillParent
	app *App
}

func (h historyView) Keybinds() []t.Keybind {
	a := h.app
	return []t.Keybind{
		{Key: "backspace", Name: "Delete entry", Action: a.deleteHistoryAtCursor},
	}
}

func (h historyView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := h.app
	if len(a.history.Get()) == 0 {
		return emptyState{Title: "No responses yet", Lines: []string{"Responses you receive are kept here so you can revisit them"}}
	}
	focused := isFocusedID(ctx, historyID)
	return t.Scrollable{
		State: a.historyScroll,
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Child: t.List[model.HistoryEntry]{
			ID:          historyID,
			State:       a.historyList,
			ScrollState: a.historyScroll,
			OnSelect:    a.openHistory,
			// Clicks open entries, like requests in the collection.
			ActivateOnClick: true,
			RenderItem: func(entry model.HistoryEntry, active, selected bool) t.Widget {
				return renderHistoryItem(theme, entry, active, focused)
			},
			Style: t.Style{Width: t.Flex(1)},
		},
	}
}

func renderHistoryItem(theme t.ThemeData, entry model.HistoryEntry, active, focused bool) t.Widget {
	var bg t.Color
	fg := theme.TextMuted
	methodFg := methodColor(theme, entry.Request.Method)
	statusFg := theme.TextMuted
	if entry.Response != nil {
		statusFg, _ = statusColors(theme, entry.Response.StatusCode)
	}
	switch {
	case active && focused:
		bg = theme.ActiveCursor
		fg, methodFg, statusFg = theme.SelectionText, theme.SelectionText, theme.SelectionText
	case active:
		bg = theme.Surface
	}
	status := t.Span{}
	if entry.Response != nil {
		status = t.Span{Text: fmt.Sprintf(" %d", entry.Response.StatusCode), Style: t.SpanStyle{Foreground: statusFg, Background: bg, Bold: true}}
	}
	target := entry.Request.URL
	if entry.Response != nil && entry.Response.URL != "" {
		target = entry.Response.URL
	}
	target = strings.TrimPrefix(strings.TrimPrefix(target, "https://"), "http://")
	return t.Column{
		Style: t.Style{Width: t.Flex(1), BackgroundColor: bg, Padding: t.EdgeInsetsXY(1, 0)},
		Children: []t.Widget{
			t.Text{Spans: []t.Span{
				{Text: string(entry.Request.Method), Style: t.SpanStyle{Foreground: methodFg, Background: bg, Bold: true}},
				status,
				{Text: " · " + entry.SentAt.Format("02 Jan 15:04:05"), Style: t.SpanStyle{Foreground: fg, Background: bg}},
			}},
			t.Text{Content: target, Style: t.Style{ForegroundColor: fg, Width: t.Flex(1)}},
		},
	}
}

// openHistory opens a history entry's request and response in a tab.
func (a *App) openHistory(entry model.HistoryEntry) {
	req := entry.Request.Clone()
	req.File = ""
	s := a.current()
	if s == nil || !s.isPristine() {
		s = a.openSession(req)
	} else {
		s.Load(req)
	}
	entryCopy := entry
	s.showResponse(entry.Response, &entryCopy)
	s.phase.Set(exchangeDone)
	s.responseTab.Set("body")
}

func (a *App) deleteHistoryAtCursor() {
	entry, ok := a.historyList.SelectedItem()
	if !ok {
		return
	}
	var kept []model.HistoryEntry
	for _, e := range a.history.Peek() {
		if e.ID != entry.ID {
			kept = append(kept, e)
		}
	}
	a.setHistory(kept)
}

func (a *App) clearHistory() {
	a.setHistory(nil)
	a.notify("History cleared", toastInfo)
}

func (a *App) duplicateAtCursor() {
	if item, ok := a.tree.CursorNode(); ok && item.Request != nil {
		a.duplicateRequest(*item.Request)
	}
}

func (a *App) fileExists(file string) bool {
	found := false
	a.collection.Peek().Walk(func(_ *model.Collection, r model.Request) {
		if r.File == file {
			found = true
		}
	})
	return found
}

func (a *App) confirmDeleteAtCursor() {
	if item, ok := a.tree.CursorNode(); ok && item.Request != nil {
		a.confirmDelete(*item.Request)
	}
}
