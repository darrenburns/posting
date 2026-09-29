package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const (
	// treeViewportID is the scrollable viewport of the request tree.
	treeViewportID = "side-tree-viewport"
	summaryID      = "side-summary"
	// summaryWidth is the widest a line of the summary bubble may be, and
	// summaryMinWidth the narrowest worth showing. summaryLines is the most
	// lines of description it shows.
	summaryWidth    = 48
	summaryMinWidth = 24
	summaryLines    = 8
)

// requestSummary floats a summary of the request under the tree cursor beside
// its row: the method and name, the URL and the description. It shows only
// while the tree has focus and the request has a description. It is only a
// preview: the pointer passes through it to whatever it covers, and clicking
// there moves focus out of the tree, which puts the bubble away.
type requestSummary struct{ app *App }

func (s requestSummary) Build(ctx t.BuildContext) t.Widget {
	a := s.app
	_ = a.tree.CursorPath.Get()
	item, ok := a.tree.CursorNode()
	if !ok || item.Request == nil || strings.TrimSpace(item.Request.Description) == "" ||
		!isFocusedID(ctx, treeID) || a.jump.IsActive() {
		return t.EmptyWidget{}
	}
	// It clears the sidebar's padding and the divider beside the tree's
	// viewport, and starts a row up so its title can line up with the top
	// row (see summaryBubble).
	anchor, x := t.AnchorRightTop, 2
	if a.settings.CollectionBrowser.Position == "right" {
		anchor, x = t.AnchorLeftTop, -3
	}
	r := *item.Request
	return t.Floating{
		Visible: true,
		Config: t.FloatConfig{
			AnchorID:           treeViewportID,
			Anchor:             anchor,
			Offset:             t.Offset{X: x, Y: -1},
			PointerPassthrough: true,
			// Without OnDismiss, the bubble never takes Escape from the
			// tree.
		},
		BuildChild: func(ctx t.BuildContext, geometry t.FloatGeometry) t.Widget {
			return summaryBubble(ctx, a, r, anchor == t.AnchorLeftTop, geometry)
		},
	}
}

// summaryBubble lays out the summary of r for this frame's tree viewport,
// the anchor in geometry. The float starts a row above the viewport, and the
// bubble's top margin moves it down to its row.
func summaryBubble(ctx t.BuildContext, a *App, r model.Request, leftOfTree bool, geometry t.FloatGeometry) t.Widget {
	theme := ctx.Theme()
	viewport := geometry.AnchorBounds
	if !geometry.AnchorFound || geometry.AnchorVisibleBounds.IsEmpty() {
		return nil
	}
	// The row's line in the viewport. The mouse wheel scrolls without moving
	// the cursor, and a row scrolled out of view has no bubble.
	row, ok := a.treeRow(a.tree.CursorPath.Get())
	top := row - a.treeScroll.Offset.Get()
	if !ok || top < 0 || top >= viewport.Height {
		return nil
	}
	// The bubble covers the edge of the workspace, short of its far side.
	// Where the workspace is too narrow for a readable line, it stays away.
	reach := geometry.Screen.Width - (viewport.X + viewport.Width + 2)
	if leftOfTree {
		reach = viewport.X - 3
	}
	width := min(summaryWidth, reach-6)
	if width < summaryMinWidth {
		return nil
	}

	lines := wrapWords(strings.TrimSpace(r.Description), width, summaryLines)
	// The title lines up with the row, below the bubble's top border. If the
	// bubble would run off the bottom of the viewport it opens upwards
	// instead, with its last line beside the row.
	height := len(lines) + 4
	if r.URL != "" {
		height++
	}
	y := top - 1
	if top-1+height > viewport.Height && top+2 >= height {
		y = top + 2 - height
	}

	line := func(spans ...t.Span) t.Widget {
		if len(spans) == 0 {
			// An empty Text has no height.
			spans = []t.Span{{Text: " "}}
		}
		return t.Text{Spans: spans}
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
	return t.Column{
		ID: summaryID,
		Style: t.Style{
			BackgroundColor: theme.Background,
			Border:          t.RoundedBorder(theme.Border),
			Padding:         t.EdgeInsetsXY(1, 0),
			Margin:          t.EdgeInsets{Top: y + 1},
		},
		Children: children,
	}
}

// treeRow is the line of the tree row at path, counting the rows shown above
// it: those of expanded folders or, while searching, the results.
func (a *App) treeRow(path []int) (int, bool) {
	// visibleTreePaths only peeks, so subscribe to what decides the rows.
	a.tree.Nodes.Get()
	a.tree.Collapsed.Get()
	a.treeFilter.Query.Get()
	for row, p := range a.visibleTreePaths() {
		if slices.Equal(p, path) {
			return row, true
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
