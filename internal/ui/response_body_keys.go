package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	t "github.com/darrenburns/terma"
)

// The response body viewer has Posting 2's keys: Vim-style movement, shift
// or visual mode to select, and y to copy.

// bodyMotion moves the response body cursor. Its keys extend the selection
// in visual mode and clear it otherwise; its selectKeys always extend it.
type bodyMotion struct {
	keys       []string
	selectKeys []string
	move       func(*t.TextAreaState)
}

var bodyMotions = []bodyMotion{
	{[]string{"up", "k"}, []string{"shift+up", "K"}, (*t.TextAreaState).CursorUp},
	{[]string{"down", "j"}, []string{"shift+down", "J"}, (*t.TextAreaState).CursorDown},
	{[]string{"left", "h"}, []string{"shift+left", "H"}, (*t.TextAreaState).CursorLeft},
	{[]string{"right", "l"}, []string{"shift+right", "L"}, (*t.TextAreaState).CursorRight},
	{[]string{"ctrl+left", "alt+b", "b"}, []string{"ctrl+shift+left", "B"}, (*t.TextAreaState).CursorWordLeft},
	{[]string{"ctrl+right", "alt+f", "w"}, []string{"ctrl+shift+right", "W"}, (*t.TextAreaState).CursorWordRight},
	{[]string{"home", "ctrl+a", "0", "^"}, []string{"shift+home"}, cursorLineStart},
	{[]string{"end", "ctrl+e", "$"}, []string{"shift+end"}, (*t.TextAreaState).CursorEnd},
	{[]string{"g"}, nil, cursorTop},
	{[]string{"G"}, nil, cursorBottom},
	{[]string{"%"}, nil, cursorMatchingBracket},
}

// responseBodyKeybinds are the response body's keys, ahead of the text
// area's own. escape leaves visual mode, and is otherwise left to cancel a
// request in flight.
func (s *Session) responseBodyKeybinds(copyBody func(), visual bool) []t.Keybind {
	binds := []t.Keybind{
		{Key: "y", Name: "Copy", Action: copyBody},
		{Key: "c", Action: copyBody, Hidden: true},
		{Key: "v", Name: "Visual", Action: func() { s.setBodyVisual(!s.responseVisual.Peek()) }},
		{Key: "V", Name: "Select line", Action: s.selectBodyLine, Hidden: true},
		{Key: "f6", Action: s.selectBodyLine, Hidden: true},
		{Key: "f7", Action: s.selectAllBody, Hidden: true},
	}
	if visual {
		binds = append(binds, t.Keybind{Key: "escape", Name: "Normal", Action: func() { s.setBodyVisual(false) }})
	}
	for _, motion := range bodyMotions {
		for _, key := range motion.keys {
			binds = append(binds, t.Keybind{Key: key, Action: s.moveBodyCursor(motion.move, false), Hidden: true})
		}
		for _, key := range motion.selectKeys {
			binds = append(binds, t.Keybind{Key: key, Action: s.moveBodyCursor(motion.move, true), Hidden: true})
		}
	}
	for _, page := range []struct {
		keys               []string
		direction, divisor int
	}{
		{[]string{"pgup", "ctrl+b"}, -1, 1},
		{[]string{"pgdown", "ctrl+f"}, 1, 1},
		{[]string{"ctrl+u"}, -1, 2},
		{[]string{"ctrl+d"}, 1, 2},
	} {
		move := func(body *t.TextAreaState) {
			lines := max(1, (s.responseBodyViewportHeight-1)/page.divisor)
			body.CursorDownBy(page.direction * lines)
		}
		for _, key := range page.keys {
			binds = append(binds, t.Keybind{Key: key, Action: s.moveBodyCursor(move, false), Hidden: true})
		}
		if page.divisor == 1 {
			binds = append(binds, t.Keybind{Key: "shift+" + page.keys[0], Action: s.moveBodyCursor(move, true), Hidden: true})
		}
	}
	return binds
}

func (s *Session) moveBodyCursor(move func(*t.TextAreaState), extend bool) func() {
	return func() {
		body := s.responseBody
		if extend || s.responseVisual.Peek() {
			if body.SelectionAnchor.Peek() < 0 {
				body.SetSelectionAnchor(body.CursorIndex.Peek())
			}
		} else {
			body.ClearSelection()
		}
		move(body)
		s.revealBodyCursor()
	}
}

// revealBodyCursor scrolls the cursor into view as the key is handled, as
// Terma's own text area keys do. The text area also reveals the cursor when
// it renders, but by then the frame is laid out at the old scroll position,
// so a jump like G shows nothing until the next keypress. In the Scrollable
// the text area never scrolls itself, so the cursor's y is its line.
func (s *Session) revealBodyCursor() {
	_, line := s.responseBody.CursorScreenPosition(0, 0)
	s.responseBodyScroll.ScrollToView(line, 1)
}

// setBodyVisual turns visual mode on or off. Turning it on anchors a
// selection at the cursor unless one is already under way; turning it off
// drops the selection.
func (s *Session) setBodyVisual(on bool) {
	body := s.responseBody
	if on && body.SelectionAnchor.Peek() < 0 {
		body.SetSelectionAnchor(body.CursorIndex.Peek())
	}
	if !on {
		body.ClearSelection()
	}
	s.responseVisual.Set(on)
}

// Whole-line and whole-body selections end past the text they select, which
// visual mode's copy would overshoot, so they leave visual mode.
func (s *Session) selectBodyLine() {
	s.responseVisual.Set(false)
	s.responseBody.SelectLine(s.responseBody.CursorIndex.Peek())
	s.revealBodyCursor()
}

func (s *Session) selectAllBody() {
	s.responseVisual.Set(false)
	s.responseBody.SelectAll()
	s.revealBodyCursor()
}

// bodyCopyText is what y copies: the selection, or the whole body when
// nothing is selected. Visual selections include both endpoint characters.
func (s *Session) bodyCopyText() (text string, selected bool) {
	body := s.responseBody
	if s.responseVisual.Peek() && body.SelectionAnchor.Peek() >= 0 {
		graphemes := body.Content.Peek()
		start, end := s.bodyVisualBounds()
		text = strings.Join(graphemes[start:end], "")
	} else {
		text = body.GetSelectedText()
	}
	if text == "" {
		return body.GetText(), false
	}
	return text, true
}

func (s *Session) bodyVisualBounds() (start, end int) {
	body := s.responseBody
	anchor, cursor := body.SelectionAnchor.Peek(), body.CursorIndex.Peek()
	length := len(body.Content.Peek())
	return min(min(anchor, cursor), length), min(max(anchor, cursor)+1, length)
}

// Terma selects half-open ranges. Visual mode also highlights the upper
// endpoint, including the anchor when selecting backwards or after blur.
func (s *Session) withBodyVisualSelection(base t.Highlighter, color t.Color) t.Highlighter {
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		out := base.Highlight(text, graphemes)
		if s.responseVisual.Peek() && s.responseBody.SelectionAnchor.Peek() >= 0 {
			start, end := s.bodyVisualBounds()
			if start < end {
				i := end - 1
				style := t.SpanStyle{}
				for _, h := range out {
					if h.Start <= i && i < h.End {
						style = h.Style
					}
				}
				style.Background = color
				out = append(out, t.TextHighlight{Start: i, End: end, Style: style})
			}
		}
		return out
	})
}

// Terma v0.19 has no indexed jump that updates its preferred column.
// Finish with a public motion so the next vertical move uses this column.
func setBodyCursor(body *t.TextAreaState, index int) {
	if index == 0 {
		body.CursorIndex.Set(0)
		body.CursorHome()
		return
	}
	body.CursorIndex.Set(index - 1)
	body.CursorRight()
}

// copyBody copies bodyCopyText, leaves visual mode, and says what it copied.
func (s *Session) copyBody() string {
	text, selected := s.bodyCopyText()
	t.SetClipboard(t.SystemClipboard, text)
	if s.responseVisual.Peek() {
		s.setBodyVisual(false)
	}
	if selected {
		return "Copied " + pluralize(utf8.RuneCountInString(text), "character")
	}
	return "Copied response body"
}

// cursorLineStart is a smart home: the first non-blank character on the
// line, or the start of the line if the cursor is already there.
func cursorLineStart(body *t.TextAreaState) {
	cursor := body.CursorIndex.Peek()
	body.CursorHome()
	start := body.CursorIndex.Peek()
	graphemes := body.Content.Peek()
	first := start
	for first < len(graphemes) && graphemes[first] != "\n" && strings.TrimSpace(graphemes[first]) == "" {
		first++
	}
	if first < len(graphemes) && graphemes[first] == "\n" {
		first = start
	}
	if cursor == start || cursor > first {
		setBodyCursor(body, first)
	}
}

func cursorTop(body *t.TextAreaState) {
	setBodyCursor(body, 0)
}

// cursorBottom goes to the start of the last line.
func cursorBottom(body *t.TextAreaState) {
	body.CursorIndex.Set(len(body.Content.Peek()))
	body.CursorHome()
}

// cursorMatchingBracket jumps to the bracket matching the one under the
// cursor, or else the first bracket later on the line that has a match.
func cursorMatchingBracket(body *t.TextAreaState) {
	graphemes := body.Content.Peek()
	for i := body.CursorIndex.Peek(); i < len(graphemes) && graphemes[i] != "\n"; i++ {
		if match := matchingBracket(graphemes, i); match >= 0 {
			setBodyCursor(body, match)
			return
		}
	}
}

var bracketPairs = map[string]string{"(": ")", "[": "]", "{": "}", ")": "(", "]": "[", "}": "{"}

// matchingBracket is the index of the bracket that pairs with the one at i,
// or -1. Like Posting 2, it counts brackets without regard to strings.
func matchingBracket(graphemes []string, i int) int {
	if i < 0 || i >= len(graphemes) {
		return -1
	}
	open := graphemes[i]
	closer, ok := bracketPairs[open]
	if !ok {
		return -1
	}
	step := 1
	if open == ")" || open == "]" || open == "}" {
		step = -1
	}
	depth := 0
	for j := i; j >= 0 && j < len(graphemes); j += step {
		switch graphemes[j] {
		case open:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// withBracketMatch emboldens the bracket under the cursor and its match, on
// top of base's highlighting.
func withBracketMatch(base t.Highlighter, body *t.TextAreaState) t.Highlighter {
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		out := base.Highlight(text, graphemes)
		cursor := body.CursorIndex.Peek()
		match := matchingBracket(graphemes, cursor)
		if match < 0 {
			return out
		}
		for _, i := range []int{cursor, match} {
			style := t.SpanStyle{}
			for _, h := range out {
				if h.Start <= i && i < h.End {
					style = h.Style
				}
			}
			style.Bold, style.Underline = true, t.UnderlineSingle
			out = append(out, t.TextHighlight{Start: i, End: i + 1, Style: style})
		}
		return out
	})
}

// cursorPosition is the cursor's 1-based line and column.
func cursorPosition(graphemes []string, cursor int) string {
	line, col := 1, 1
	for _, g := range graphemes[:min(cursor, len(graphemes))] {
		if g == "\n" {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return fmt.Sprintf("%d:%d", line, col)
}
