package ui

import (
	"strings"
	"unicode"

	t "github.com/darrenburns/terma"
)

// smartInput supplies editing policy for request payloads. Terma owns applying
// the edit, its undo step, change notification, and cursor reveal. Pasted text
// never passes through this hook.
func smartInput(state *t.TextAreaState, language string) func(t.TextAreaEdit) t.TextAreaEdit {
	return func(edit t.TextAreaEdit) t.TextAreaEdit {
		switch edit.Text {
		case "\n", `"`, "'", "`", "(", ")", "[", "]", "{", "}":
		default:
			return edit
		}
		content := state.Content.Peek()
		start, end := edit.Start, edit.End
		context := inputContext(content[:start], language)
		line := start
		for line > 0 && content[line-1] != "\n" {
			line--
		}
		indentEnd := line
		for indentEnd < start && inputSpace(content[indentEnd]) {
			indentEnd++
		}
		indent := strings.Join(content[line:indentEnd], "")
		if edit.Text == "\n" {
			edit.Text += indent
			last := start - 1
			for last >= line && inputSpace(content[last]) {
				last--
			}
			if language != "" && context.quote == "" && !context.comment && last >= line {
				closer := inputCloser(content[last], language)
				if strings.Contains("([{", content[last]) || (language == "yaml" && content[last] == ":") {
					edit.Text += inputIndent(content)
					// Put the existing closer on its own line, consuming only
					// horizontal whitespace between it and the cursor.
					next := end
					for next < len(content) && inputSpace(content[next]) {
						next++
					}
					edit.Cursor = start + len([]rune(edit.Text))
					if closer != "" && next < len(content) && content[next] == closer {
						edit.End = next
						edit.Text += "\n" + indent
					}
					return edit
				}
			}
			edit.Cursor = start + len([]rune(edit.Text)) // Only newlines, spaces and tabs.
			return edit
		}
		if language == "" {
			return edit
		}
		closer := inputCloser(edit.Text, language)
		if start != end && closer != "" {
			edit.Text += strings.Join(content[start:end], "") + closer
			edit.Cursor = state.CursorIndex.Peek() + 1
			edit.SelectionAnchor = state.SelectionAnchor.Peek() + 1
			return edit
		}
		if context.comment || context.escaped {
			return edit
		}
		if context.quote != "" {
			if start == end && edit.Text == context.quote && end < len(content) && content[end] == edit.Text {
				edit.Text = "" // Move through a closing quote, not an opening one.
			}
			return edit
		}
		if start == end && strings.Contains(")]}", edit.Text) && len(context.brackets) > 0 {
			opener := context.brackets[len(context.brackets)-1]
			if inputCloser(content[opener], language) == edit.Text {
				if end < len(content) && content[end] == edit.Text {
					edit.Text = ""
				} else if indentEnd == start {
					// A closer typed on an otherwise empty line aligns with
					// its opening bracket, rather than the nested contents.
					openLine := opener
					for openLine > 0 && content[openLine-1] != "\n" {
						openLine--
					}
					openIndent := openLine
					for openIndent < opener && inputSpace(content[openIndent]) {
						openIndent++
					}
					edit.Start = line
					edit.Text = strings.Join(content[openLine:openIndent], "") + edit.Text
					edit.Cursor = line + openIndent - openLine + 1
				}
			}
			return edit
		}
		if closer == "" || (end < len(content) && !inputBoundary(content[end])) {
			return edit
		}
		if closer == edit.Text && end < len(content) && content[end] == edit.Text {
			return edit // Before an opening quote: insert literally, don't overtype or add a pair.
		}
		if edit.Text == "'" && start > 0 && !inputBoundary(content[start-1]) {
			return edit // An apostrophe in a word, not a string delimiter.
		}
		edit.Text += closer
		return edit
	}
}

func inputCloser(opener, language string) string {
	switch opener {
	case "(":
		return ")"
	case "[":
		return "]"
	case "{":
		return "}"
	case `"`:
		return `"`
	case "'":
		if language != "json" && language != "graphql" {
			return "'"
		}
	case "`":
		if language == "javascript" {
			return "`"
		}
	}
	return ""
}

func inputSpace(g string) bool { return g == " " || g == "\t" }

func inputBoundary(g string) bool {
	return strings.ContainsAny(g, "()[]{},:;=<>\"'`") || strings.ContainsFunc(g, unicode.IsSpace)
}

// inputIndent follows existing leading whitespace, defaulting to two spaces
// like Posting's JSON formatting. Preserve tabs rather than expanding them.
func inputIndent(content []string) string {
	unit := ""
	for i := 0; i < len(content); i++ {
		start := i
		for i < len(content) && inputSpace(content[i]) {
			if content[i] == "\t" {
				return "\t"
			}
			i++
		}
		if i > start && i < len(content) && content[i] != "\n" && (unit == "" || i-start < len(unit)) {
			unit = strings.Repeat(" ", i-start)
		}
		for i < len(content) && content[i] != "\n" {
			i++
		}
	}
	if unit == "" {
		return "  "
	}
	return unit
}

// inputContext scans the prefix rather than tracking generated delimiter
// positions, so loaded text, undo/redo and external edits need no bookkeeping.
// It is deliberately lexical: pairing must not fire in strings or comments.
func inputContext(content []string, language string) (ctx struct {
	quote    string
	escaped  bool
	comment  bool
	brackets []int
}) {
	blockEnd := ""
	for i := 0; i < len(content); i++ {
		g := content[i]
		next := ""
		if i+1 < len(content) {
			next = content[i+1]
		}
		if ctx.comment {
			if blockEnd == "" && g == "\n" {
				ctx.comment = false
			} else if blockEnd == "*/" && g+next == blockEnd {
				ctx.comment, blockEnd = false, ""
				i++
			} else if blockEnd == "-->" && i+2 < len(content) && g+next+content[i+2] == blockEnd {
				ctx.comment, blockEnd = false, ""
				i += 2
			}
			continue
		}
		if ctx.quote != "" {
			if ctx.escaped {
				ctx.escaped = false
			} else if g == "\\" && language != "xml" && language != "html" && !(language == "yaml" && ctx.quote == "'") {
				ctx.escaped = true
			} else if ctx.quote == `"""` {
				if i+2 < len(content) && g+next+content[i+2] == ctx.quote {
					ctx.quote = ""
					i += 2
				}
			} else if g == ctx.quote {
				ctx.quote = ""
			}
			continue
		}
		switch {
		case (language == "graphql" || language == "yaml") && g == "#":
			ctx.comment = true
		case language == "javascript" && g+next == "//":
			ctx.comment = true
			i++
		case (language == "javascript" || language == "css") && g+next == "/*":
			ctx.comment, blockEnd = true, "*/"
			i++
		case (language == "xml" || language == "html") && i+3 < len(content) && g+next+content[i+2]+content[i+3] == "<!--":
			ctx.comment, blockEnd = true, "-->"
			i += 3
		case g == `"` || ((g == "'" || g == "`") && inputCloser(g, language) != ""):
			if g == "'" && language == "yaml" && i > 0 && !inputBoundary(content[i-1]) {
				continue // Apostrophe in an unquoted YAML scalar.
			}
			ctx.quote = g
			if language == "graphql" && i+2 < len(content) && g+next+content[i+2] == `"""` {
				ctx.quote = `"""`
				i += 2
			}
		case strings.Contains("([{", g):
			ctx.brackets = append(ctx.brackets, i)
		case strings.Contains(")]}", g) && len(ctx.brackets) > 0:
			last := len(ctx.brackets) - 1
			if inputCloser(content[ctx.brackets[last]], language) == g {
				ctx.brackets = ctx.brackets[:last]
			}
		}
	}
	return ctx
}
