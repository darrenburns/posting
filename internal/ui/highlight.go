package ui

import (
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

// byteToGrapheme maps byte offsets in text to grapheme indices.
func byteToGrapheme(graphemes []string, textLen int) []int {
	index := make([]int, textLen+1)
	pos := 0
	for i, g := range graphemes {
		for b := 0; b < len(g) && pos+b <= textLen; b++ {
			index[pos+b] = i
		}
		pos += len(g)
	}
	if pos <= textLen {
		index[pos] = len(graphemes)
	}
	return index
}

func span(index []int, start, end int, style t.SpanStyle) t.TextHighlight {
	return t.TextHighlight{Start: index[start], End: index[end], Style: style}
}

// variableHighlights colours ${VAR} references green when they resolve and
// red when they don't.
func variableHighlights(theme t.ThemeData, text string, index []int, resolve func(string) bool) []t.TextHighlight {
	return refHighlights(theme, model.FindVariables(text), index, resolve)
}

// refHighlights colours the given variable references as variableHighlights does.
func refHighlights(theme t.ThemeData, refs []model.VariableRef, index []int, resolve func(string) bool) []t.TextHighlight {
	var out []t.TextHighlight
	for _, ref := range refs {
		style := t.SpanStyle{Foreground: theme.SuccessText}
		if resolve != nil && !resolve(ref.Name) {
			style = t.SpanStyle{Foreground: theme.ErrorText, Underline: t.UnderlineCurly, UnderlineColor: theme.Error}
		}
		out = append(out, span(index, ref.Start, ref.End, style))
	}
	return out
}

// variableHighlighter highlights variable references in any single-line value.
func variableHighlighter(theme t.ThemeData, resolve func(string) bool) t.Highlighter {
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		if !strings.Contains(text, "$") {
			return nil
		}
		return variableHighlights(theme, text, byteToGrapheme(graphemes, len(text)), resolve)
	})
}

// urlHighlighter colours the scheme, host, separators, :path params and variables.
func urlHighlighter(theme t.ThemeData, resolve func(string) bool) t.Highlighter {
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		index := byteToGrapheme(graphemes, len(text))
		var out []t.TextHighlight
		muted := t.SpanStyle{Foreground: theme.TextMuted}
		rest := 0
		if i := strings.Index(text, "://"); i >= 0 {
			out = append(out, span(index, 0, i, t.SpanStyle{Foreground: theme.AccentText}))
			out = append(out, span(index, i, i+3, muted))
			rest = i + 3
			hostEnd := len(text)
			if j := strings.IndexAny(text[rest:], "/?#"); j >= 0 {
				hostEnd = rest + j
			}
			out = append(out, span(index, rest, hostEnd, t.SpanStyle{Foreground: theme.SecondaryText}))
			rest = hostEnd
		}
		queryStart := len(text)
		if q := strings.IndexByte(text, '?'); q >= rest {
			queryStart = q
		}
		for i := rest; i < queryStart; i++ {
			if text[i] != '/' {
				continue
			}
			out = append(out, span(index, i, i+1, muted))
			if i+1 < len(text) && text[i+1] == ':' && (i+2 >= len(text) || text[i+2] != ':') {
				end := i + 2
				for end < queryStart && text[end] != '/' {
					end++
				}
				out = append(out, span(index, i+1, end, t.SpanStyle{Foreground: theme.InfoText, Bold: true}))
			}
		}
		for i := queryStart; i < len(text); i++ {
			if c := text[i]; c == '?' || c == '&' || c == '=' {
				out = append(out, span(index, i, i+1, muted))
			}
		}
		return append(out, variableHighlights(theme, text, index, resolve)...)
	})
}

// syntaxHighlighter highlights text areas with chroma. Tokenising happens
// once per distinct text; the result is cached.
type syntaxHighlighter struct {
	theme    t.ThemeData
	language string

	mu     sync.Mutex
	text   string
	result []t.TextHighlight
}

func newSyntaxHighlighter(theme t.ThemeData, language string) *syntaxHighlighter {
	return &syntaxHighlighter{theme: theme, language: language}
}

func (h *syntaxHighlighter) Highlight(text string, graphemes []string) []t.TextHighlight {
	if h == nil || h.language == "" || text == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if text == h.text && h.result != nil {
		return h.result
	}
	lexer := lexers.Get(h.language)
	if lexer == nil {
		return nil
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil
	}
	index := byteToGrapheme(graphemes, len(text))
	var out []t.TextHighlight
	pos := 0
	for _, token := range iterator.Tokens() {
		end := pos + len(token.Value)
		if end > len(text) {
			break
		}
		if style, ok := h.styleFor(token.Type); ok && end > pos {
			out = append(out, span(index, pos, end, style))
		}
		pos = end
	}
	h.text, h.result = text, out
	return out
}

func (h *syntaxHighlighter) styleFor(tt chroma.TokenType) (t.SpanStyle, bool) {
	theme := h.theme
	switch {
	case tt == chroma.NameTag, tt == chroma.NameAttribute:
		return t.SpanStyle{Foreground: theme.PrimaryText}, true
	case tt.InCategory(chroma.Comment):
		return t.SpanStyle{Foreground: theme.TextMuted, Italic: true}, true
	case tt.InCategory(chroma.LiteralString):
		return t.SpanStyle{Foreground: theme.SuccessText}, true
	case tt.InCategory(chroma.LiteralNumber):
		return t.SpanStyle{Foreground: theme.WarningText}, true
	case tt.InCategory(chroma.Keyword), tt == chroma.NameBuiltin:
		return t.SpanStyle{Foreground: theme.AccentText}, true
	case tt.InCategory(chroma.Name):
		return t.SpanStyle{Foreground: theme.InfoText}, true
	case tt.InCategory(chroma.Punctuation), tt.InCategory(chroma.Operator):
		return t.SpanStyle{Foreground: theme.TextMuted}, true
	}
	return t.SpanStyle{}, false
}

// languageFor maps a content type to a chroma lexer name.
func languageFor(contentType string) string {
	switch {
	case strings.Contains(contentType, "json"):
		return "json"
	case strings.Contains(contentType, "html"):
		return "html"
	case strings.Contains(contentType, "xml"):
		return "xml"
	case strings.Contains(contentType, "css"):
		return "css"
	case strings.Contains(contentType, "javascript"):
		return "javascript"
	case strings.Contains(contentType, "yaml"):
		return "yaml"
	}
	return ""
}
