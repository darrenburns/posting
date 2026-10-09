package ui

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

const (
	// treeViewportID is the scrollable viewport of the request tree.
	treeViewportID = "side-tree-viewport"
	summaryID      = "side-summary"
	// summaryAnchorID is the top row of the tree's viewport, which the
	// summary hangs from.
	summaryAnchorID = "side-summary-anchor"
	// summaryWidth is the widest a line of the summary bubble may be, and
	// summaryMinWidth the narrowest worth showing. summaryLines is the most
	// lines of description it shows.
	summaryWidth    = 48
	summaryMinWidth = 24
	summaryLines    = 8
)

// summaryDwell is how long the pointer rests on a row of the tree before the
// summary of its request shows.
const summaryDwell = 500 * time.Millisecond

// summaryState decides which request the summary describes, if any.
//
// Resting the pointer on a row shows the summary of that row's request. Moving
// the tree cursor with the keyboard shows the cursor's, while the tree has
// focus. Clicking a row, or opening one, puts the summary away until the
// cursor next moves by keyboard; so does the pointer taking over, once a
// hovered row's summary has shown.
type summaryState struct {
	// hovered is the row the pointer has rested on for summaryDwell, or nil.
	hovered t.AnySignal[[]int]
	// muted hides the summary of the tree cursor.
	muted t.Signal[bool]
	// pressing is set while the tree handles a press on one of its rows, so
	// the cursor move the press makes isn't taken for keyboard movement.
	pressing bool
	// dwell counts changes of hover. A dwell overtaken by one does nothing.
	dwell int
}

func newSummaryState() summaryState {
	return summaryState{hovered: t.NewAnySignal[[]int](nil), muted: t.NewSignal(false)}
}

// trackTreeRow has row, the tree row at path, report the pointer to the
// summary.
func (a *App) trackTreeRow(row t.Text, path []int) t.Text {
	s := &a.summary
	row.Hover = func(e t.HoverEvent) {
		s.dwell++
		if e.Type == t.HoverLeave {
			if slices.Equal(s.hovered.Peek(), path) {
				s.hovered.Set(nil)
			}
			return
		}
		// Only the pointer moving onto a row starts a dwell. Rows moving
		// under a still pointer, as the tree scrolls, don't.
		if e.Source != t.HoverSourcePointer {
			return
		}
		dwell := s.dwell
		a.after(summaryDwell, func() {
			if s.dwell == dwell {
				s.hovered.Set(path)
				s.muted.Set(true)
			}
		})
	}
	// The row hears of a press before the tree moves its cursor, and of the
	// click after.
	row.MouseDown = func(t.MouseEvent) {
		s.pressing = true
		a.dismissSummary()
	}
	click := row.Click
	row.Click = func(e t.MouseEvent) {
		s.pressing = false
		if click != nil {
			click(e)
		}
	}
	return row
}

// treeCursorMoved shows the summary of the request under the tree cursor
// when the keyboard moved it.
func (a *App) treeCursorMoved() {
	s := &a.summary
	if s.pressing {
		return
	}
	s.dwell++
	s.hovered.Set(nil)
	s.muted.Set(false)
}

// dismissSummary puts the summary away until the tree cursor next moves by
// keyboard.
func (a *App) dismissSummary() {
	s := &a.summary
	s.dwell++
	s.hovered.Set(nil)
	s.muted.Set(true)
}

// requestSummary floats a summary of a request beside its row in the tree:
// the method and name, the URL and the description. It shows only for a
// request with a description, and describes the row the pointer rests on or,
// while the tree has focus, the one under the tree cursor (see
// summaryState). It is only a preview: the pointer passes through it to
// whatever it covers.
//
// A Terma anchor places a float outside its widget, so the summary hangs
// from a strip along the top row of the tree, under it, rather than from
// the tree's viewport.
type requestSummary struct {
	app *App
	// row renders a row of the tree, which the bubble sits beside.
	row func(treeItem, t.TreeNodeContext) t.Text
}

// WidgetID names the strip the summary hangs from.
func (requestSummary) WidgetID() string { return summaryAnchorID }

func (s requestSummary) Build(ctx t.BuildContext) t.Widget {
	a := s.app
	path := a.summary.hovered.Get()
	if path == nil {
		if !isFocusedID(ctx, treeID) || a.summary.muted.Get() {
			return t.EmptyWidget{}
		}
		path = a.tree.CursorPath.Get()
	}
	if a.jump.IsActive() {
		return t.EmptyWidget{}
	}
	place := summaryPlace{row: -1, leftOfTree: a.settings.CollectionBrowser.Position == "right"}
	var r model.Request
	for i, row := range a.treeRows() {
		if slices.Equal(row.path, path) {
			if row.item.Request == nil {
				return t.EmptyWidget{}
			}
			place.row, r = i, *row.item.Request
		}
		width := spansWidth(s.row(row.item, t.TreeNodeContext{Path: row.path}).Spans)
		place.labels = append(place.labels, treeLabelColumn(len(row.path)-1)+width)
	}
	if place.row < 0 || strings.TrimSpace(r.Description) == "" {
		return t.EmptyWidget{}
	}
	// The float starts a row above the tree, so the bubble's title can line
	// up with the top row (see summaryBubble).
	anchor, offset := t.AnchorBottomLeft, t.Offset{Y: -2}
	if place.leftOfTree {
		anchor, offset = t.AnchorLeftTop, t.Offset{X: -3, Y: -1}
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)},
		Children: []t.Widget{t.Floating{
			Visible: true,
			Config: t.FloatConfig{
				AnchorID:           summaryAnchorID,
				Anchor:             anchor,
				Offset:             offset,
				PointerPassthrough: true,
				// Without OnDismiss, the bubble never takes Escape from the
				// tree.
			},
			BuildChild: func(ctx t.BuildContext, geometry t.FloatGeometry) t.Widget {
				return summaryBubble(ctx, a, r, place, geometry)
			},
		}},
	}
}

// summaryPlace is where the summary goes: beside the tree's row, the
// summarised request's, or to the left of the tree. labels holds where each
// row's label ends, in cells from the left of the tree.
type summaryPlace struct {
	row        int
	labels     []int
	leftOfTree bool
}

// treeLabelColumn is where the label of a tree row at depth starts: Terma's
// tree indents each level two cells and puts a two-cell indicator (an arrow,
// or blank for a request) before the label.
func treeLabelColumn(depth int) int { return 2*depth + 2 }

func spansWidth(spans []t.Span) int {
	width := 0
	for _, span := range spans {
		width += utf8.RuneCountInString(span.Text)
	}
	return width
}

// summaryBubble lays out the summary of r for this frame's tree viewport,
// whose top row is the anchor in geometry. The float starts a row above the
// viewport, and the bubble's margins move it to its row.
func summaryBubble(ctx t.BuildContext, a *App, r model.Request, place summaryPlace, geometry t.FloatGeometry) t.Widget {
	theme := ctx.Theme()
	if !geometry.AnchorFound {
		return nil
	}
	// The tree can run to the bottom of the screen.
	viewport := geometry.AnchorBounds
	viewport.Height = geometry.Screen.Height - viewport.Y
	// The row's line in the viewport. The mouse wheel scrolls without moving
	// the cursor, and a row scrolled out of view has no bubble.
	scroll := a.treeScroll.Offset.Get()
	top := place.row - scroll
	if top < 0 || top >= viewport.Height {
		return nil
	}

	// Beside a tree on the left, the bubble starts a cell past the labels
	// of the rows it covers, the row's own and those above and below it, so
	// it hides none of them. Past a label running to the edge of the tree,
	// it clears the divider instead. It reaches as far as the far side of
	// the workspace, short of the edge; where that is too narrow for a
	// readable line, it stays away.
	past := func(row int) int { return min(place.labels[row]+1, viewport.Width+2) }
	left := past(place.row)
	var (
		width, y int
		lines    []string
	)
	for {
		reach := geometry.Screen.Width - (viewport.X + left)
		if place.leftOfTree {
			left, reach = 0, viewport.X-3
		}
		width = min(summaryWidth, reach-6)
		if width < summaryMinWidth {
			return nil
		}
		lines = wrapWords(strings.TrimSpace(r.Description), width, summaryLines)
		// The title lines up with the row, below the bubble's top border.
		// If the bubble would run off the bottom of the viewport it opens
		// upwards instead, with its last line beside the row.
		height := len(lines) + 4
		if r.URL != "" {
			height++
		}
		y = top - 1
		if top-1+height > viewport.Height && top+2 >= height {
			y = top + 2 - height
		}
		widest := left
		for line := max(y, 0); line < min(y+height, viewport.Height, len(place.labels)-scroll); line++ {
			widest = max(widest, past(scroll+line))
		}
		if place.leftOfTree || widest == left {
			break
		}
		left = widest
	}

	line := func(spans ...t.Span) t.Widget {
		if len(spans) == 0 {
			// An empty Text has no height.
			spans = []t.Span{{Text: " "}}
		}
		return t.Text{Spans: spans}
	}
	children := []t.Widget{line(
		t.Span{Text: " " + requestLabel(r) + " ", Style: t.SpanStyle{Foreground: requestColor(theme, r), Background: theme.Surface3, Bold: true}},
		t.Span{Text: " " + truncate(r.DisplayName(), width-len(requestLabel(r))-3), Style: t.SpanStyle{Foreground: theme.Text, Bold: true}},
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
	return t.Column{
		ID: summaryID,
		Style: t.Style{
			BackgroundColor: theme.Background,
			Border:          t.RoundedBorder(theme.Border),
			Padding:         t.EdgeInsetsXY(1, 0),
			Margin:          t.EdgeInsets{Top: y + 1, Left: left},
		},
		Children: children,
	}
}

// shownTreeRow is a row of the tree: the node at path.
type shownTreeRow struct {
	path []int
	item treeItem
}

// treeRows lists the rows of the tree from the top, as the Tree lays them
// out: those of expanded folders or, while searching, the results.
func (a *App) treeRows() []shownTreeRow {
	// visibleTreePaths only peeks, so subscribe to what decides the rows.
	a.tree.Nodes.Get()
	a.tree.Collapsed.Get()
	a.treeFilter.Query.Get()
	var rows []shownTreeRow
	for _, p := range a.visibleTreePaths() {
		if node, ok := a.tree.NodeAtPath(p); ok {
			rows = append(rows, shownTreeRow{path: p, item: node.Data})
		}
	}
	return rows
}

// treeRow is the line of the tree row at path, counting the rows shown above
// it.
func (a *App) treeRow(path []int) (int, bool) {
	for i, row := range a.treeRows() {
		if slices.Equal(row.path, path) {
			return i, true
		}
	}
	return 0, false
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
