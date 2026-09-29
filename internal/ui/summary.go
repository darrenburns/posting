package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	t "github.com/darrenburns/terma"
)

const (
	treeViewportID = "side-tree-viewport"
	summaryID      = "side-summary"
	// summaryWidth is the widest a line of the summary bubble may be, and
	// summaryMinWidth the narrowest worth showing. summaryLines is the most
	// lines of description it shows.
	summaryWidth    = 48
	summaryMinWidth = 24
	summaryLines    = 8
)

// summaryState places the bubble summarising the request under the tree
// cursor, and keeps it out of the pointer's way.
type summaryState struct {
	// viewport is the height of the tree's viewport and room the width of
	// the workspace beside it. Layout measures them for the next frame.
	viewport t.Signal[int]
	room     t.Signal[int]
	// hidden is set when the pointer moves over the bubble, until the tree
	// cursor moves.
	hidden t.Signal[bool]
	// pointer is where the pointer was at the last hover event since the
	// bubble was built, if pointerKnown.
	pointerX, pointerY int
	pointerKnown       bool
}

func newSummaryState() *summaryState {
	return &summaryState{viewport: t.NewSignal(0), room: t.NewSignal(0), hidden: t.NewSignal(false)}
}

// brush hides the bubble when the pointer moves over it. Terma gives the
// pointer to whatever floats on top, so the bubble gets out of its way rather
// than cover the workspace beneath. Hover events also fire when the bubble
// appears under a pointer that is standing still, and those leave it be.
func (s *summaryState) brush(x, y int) {
	if s.pointerKnown && (x != s.pointerX || y != s.pointerY) {
		s.hidden.Set(true)
	}
	s.pointerX, s.pointerY, s.pointerKnown = x, y, true
}

// treeViewportProbe sits behind the tree's Scrollable and fills it. Terma
// doesn't tell widgets their size while they build, so it reports the
// viewport's height from layout. The summary bubble is anchored to it.
type treeViewportProbe struct{ app *App }

func (p treeViewportProbe) GetContentDimensions() (t.Dimension, t.Dimension) {
	return t.Flex(1), t.Flex(1)
}
func (p treeViewportProbe) Build(t.BuildContext) t.Widget { return p }
func (p treeViewportProbe) Render(*t.RenderContext)       {}
func (p treeViewportProbe) WidgetID() string              { return treeViewportID }

// OnLayout records the height for the next frame, as heightProbe does.
func (p treeViewportProbe) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	if height := metrics.Box().Height; height != p.app.summary.viewport.Peek() {
		t.Dispatch(func() { p.app.summary.viewport.Set(height) })
	}
}

// requestSummary floats a summary of the request under the tree cursor beside
// its row: the method and name, the URL and the description. It shows only
// while the tree has focus and the request has a description.
type requestSummary struct{ app *App }

func (s requestSummary) Build(ctx t.BuildContext) t.Widget {
	a := s.app
	state := a.summary
	theme := ctx.Theme()
	cursor := a.tree.CursorPath.Get()
	item, ok := a.tree.CursorNode()
	if !ok || item.Request == nil || strings.TrimSpace(item.Request.Description) == "" ||
		!isFocusedID(ctx, treeID) || a.jump.IsActive() || state.hidden.Get() {
		return t.EmptyWidget{}
	}
	// The row's line in the viewport. The mouse wheel scrolls without moving
	// the cursor, and a row scrolled out of view has no bubble.
	row, ok := a.treeRow(cursor)
	viewport := state.viewport.Get()
	top := row - a.treeScroll.Offset.Get()
	if !ok || top < 0 || top >= viewport {
		return t.EmptyWidget{}
	}
	// The bubble covers the edge of the workspace, short of its far side.
	// Where the workspace is too narrow for a readable line, it stays away.
	width := min(summaryWidth, state.room.Get()-5)
	if width < summaryMinWidth {
		return t.EmptyWidget{}
	}

	r := item.Request
	lines := wrapWords(strings.TrimSpace(r.Description), width, summaryLines)
	// The title lines up with the row, below the bubble's top border. If the
	// bubble would run off the bottom it opens upwards instead, with its last
	// line beside the row. Its lines are wrapped here so its height is known.
	height := len(lines) + 4
	if r.URL != "" {
		height++
	}
	y := top - 1
	if top-1+height > viewport && top+2 >= height {
		y = top + 2 - height
	}
	// It clears the sidebar's padding and the divider beside it.
	anchor, x := t.AnchorRightTop, 2
	if a.settings.CollectionBrowser.Position == "right" {
		anchor, x = t.AnchorLeftTop, -3
	}

	// The bubble may have moved, so the next hover event starts afresh.
	state.pointerKnown = false
	brush := func(e t.HoverEvent) { state.brush(e.X, e.Y) }
	press := func(t.MouseEvent) { state.hidden.Set(true) }
	line := func(spans ...t.Span) t.Widget {
		if len(spans) == 0 {
			// An empty Text has no height.
			spans = []t.Span{{Text: " "}}
		}
		return t.Text{Spans: spans, Hover: brush, MouseDown: press}
	}
	children := []t.Widget{line(
		t.Span{Text: " " + string(r.Method) + " ", Style: t.SpanStyle{Foreground: methodColor(theme, r.Method), Background: theme.Surface3, Bold: true}},
		t.Span{Text: " " + truncate(r.DisplayName(), width-len(r.Method)-3), Style: t.SpanStyle{Foreground: theme.Text, Bold: true}},
	)}
	if r.URL != "" {
		children = append(children, line(t.Span{Text: truncate(r.URL, width), Style: t.SpanStyle{Foreground: theme.TextMuted}}))
	}
	children = append(children, line())
	code := false
	for _, text := range lines {
		var spans []t.Span
		spans, code = codeSpans(theme, text, code)
		children = append(children, line(spans...))
	}

	return t.Floating{
		Visible: true,
		Config: t.FloatConfig{
			AnchorID: treeViewportID,
			Anchor:   anchor,
			Offset:   t.Offset{X: x, Y: y},
			// Without OnDismiss, the bubble never takes Escape or a click
			// outside it from the widgets beneath.
		},
		Child: t.Column{
			ID: summaryID,
			Style: t.Style{
				BackgroundColor: theme.Background,
				Border:          t.RoundedBorder(theme.Border),
				Padding:         t.EdgeInsetsXY(1, 0),
			},
			Hover:     brush,
			MouseDown: press,
			Children:  children,
		},
	}
}

// treeRow is the line of the tree row at path, counting the rows of expanded
// folders above it.
func (a *App) treeRow(path []int) (int, bool) {
	_ = a.tree.Collapsed.Get()
	row := 0
	var walk func(nodes []t.TreeNode[treeItem], prefix []int) bool
	walk = func(nodes []t.TreeNode[treeItem], prefix []int) bool {
		for i, node := range nodes {
			p := append(slices.Clip(prefix), i)
			if slices.Equal(p, path) {
				return true
			}
			row++
			if len(node.Children) > 0 && !a.tree.IsCollapsed(p) && walk(node.Children, p) {
				return true
			}
		}
		return false
	}
	return row, len(path) > 0 && walk(a.tree.Nodes.Get(), nil)
}

// wrapWords breaks text into lines of at most width runes at spaces, keeping
// its line breaks, and ends with an ellipsis if it runs past limit lines.
func wrapWords(text string, width, limit int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			for utf8.RuneCountInString(word) > width {
				if line != "" {
					lines, line = append(lines, line), ""
				}
				runes := []rune(word)
				lines, word = append(lines, string(runes[:width])), string(runes[width:])
			}
			switch {
			case line == "":
				line = word
			case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width:
				line += " " + word
			default:
				lines, line = append(lines, line), word
			}
		}
		lines = append(lines, line)
	}
	if len(lines) > limit {
		lines = lines[:limit]
		last := []rune(strings.TrimSpace(lines[limit-1]))
		if len(last) > width-2 {
			last = last[:width-2]
		}
		lines[limit-1] = string(last) + " …"
	}
	return lines
}

// codeSpans styles the `code` in a line of description, dropping the
// backticks. code says whether the line starts inside a code span, and the
// result whether the next line does.
func codeSpans(theme t.ThemeData, line string, code bool) ([]t.Span, bool) {
	var spans []t.Span
	for i, part := range strings.Split(line, "`") {
		if i > 0 {
			code = !code
		}
		if part == "" {
			continue
		}
		fg := theme.Text
		if code {
			fg = theme.AccentText
		}
		spans = append(spans, t.Span{Text: part, Style: t.SpanStyle{Foreground: fg}})
	}
	return spans, code
}
