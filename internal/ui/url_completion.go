package ui

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
	"github.com/rivo/uniseg"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
)

// urlTree shares prefixes by URL segment, not by character. Empty segments
// are significant: /users, /users/ and /users//posts are different paths.
type urlTree map[string]urlTree

func urlParts(url string) []string {
	url, _, _ = strings.Cut(url, "?")
	url, _, _ = strings.Cut(url, "#")
	start := 0
	if i := strings.Index(url, "://"); i >= 0 {
		start = i + 3
	}
	if i := strings.IndexByte(url[start:], '/'); i >= 0 {
		i += start
		return append([]string{url[:i]}, strings.Split(url[i+1:], "/")...)
	}
	return []string{url}
}

func (a *App) collectionURLs() urlTree {
	root := a.collection.Get()
	if a.urlTree == nil {
		a.urlTree = urlTree{}
		root.Walk(func(_ *model.Collection, r model.Request) {
			if r.URL == "" {
				return
			}
			node := a.urlTree
			for _, part := range urlParts(r.URL) {
				if node[part] == nil {
					node[part] = urlTree{}
				}
				node = node[part]
			}
		})
	}
	return a.urlTree
}

type urlChoice struct {
	label, query string
	start, end   int // byte offsets; TextInput's cursor uses graphemes
}

// suggestions returns already filtered choices. Labels include the preceding
// URL so Terma's full-input filter retains them; the popup renders only the
// segment, and insertion replaces its complete range, including after the cursor.
func (tree urlTree) suggestions(text string, cursor int, variables variableChoices) []t.Suggestion {
	graphemes := uniseg.NewGraphemes(text)
	pos := 0
	for i := 0; i < cursor && graphemes.Next(); i++ {
		_, pos = graphemes.Positions()
	}
	before := text[:pos]
	if before == "" || curl.IsCommand(text) || strings.ContainsFunc(before, unicode.IsSpace) {
		return nil
	}
	// An unfinished variable takes priority, even in a query string. Completed
	// ${BASE_URL} references are ordinary, literal tree segments.
	if start := strings.LastIndexByte(before, '$'); start >= 0 {
		if strings.HasSuffix(before[:start], "${") {
			start -= 2 // The inner $ of ${$uuid} belongs to the same reference.
		}
		query := before[start+1:]
		name := strings.TrimPrefix(strings.TrimPrefix(query, "{"), "$")
		if !strings.ContainsFunc(name, func(r rune) bool {
			return r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			end := pos
			for end < len(text) && (text[end] == '_' || text[end] >= 'A' && text[end] <= 'Z' || text[end] >= 'a' && text[end] <= 'z' || text[end] >= '0' && text[end] <= '9') {
				end++
			}
			if end < len(text) && text[end] == '}' {
				end++
			}
			var choices []t.Suggestion
			for _, v := range variables.list {
				if t.MatchString(v.Label, query, t.FilterOptions{Mode: t.FilterFuzzy}).Matched {
					v.Data = urlChoice{v.Label, query, start, end}
					v.Label = text[:start] + v.Label
					choices = append(choices, v)
				}
			}
			return choices
		}
	}
	if strings.ContainsAny(before, "?#") {
		return nil
	}
	parts := urlParts(before)
	query := parts[len(parts)-1]
	start := pos - len(query)
	end := pos
	if len(parts) == 1 {
		end = len(urlParts(text)[0])
	} else {
		for end < len(text) && !strings.ContainsRune("/?#", rune(text[end])) {
			end++
		}
	}
	node := tree
	if len(parts) > 1 {
		node = urlTree{}
		for base, branch := range tree {
			if !strings.Contains(parts[0], "://") {
				if _, host, ok := strings.Cut(base, "://"); ok {
					base = host
				}
			}
			// Hosts are case-insensitive; path segments and templates are not.
			if base != parts[0] && (strings.Contains(base+parts[0], "$") || !strings.EqualFold(base, parts[0])) {
				continue
			}
			for _, part := range parts[1 : len(parts)-1] {
				branch = branch[part]
			}
			for part := range branch {
				node[part] = nil
			}
		}
	}
	var choices []t.Suggestion
	for part := range node {
		matches := strings.HasPrefix(part, query)
		if len(parts) == 1 && !strings.Contains(part, "$") {
			candidate := part
			if !strings.Contains(query, "://") {
				if _, host, ok := strings.Cut(part, "://"); ok && strings.HasPrefix(strings.ToLower(host), strings.ToLower(query)) {
					candidate = host
				}
			}
			matches = strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(query))
		}
		if !matches || part == "" || part == text[start:end] {
			continue
		}
		choices = append(choices, t.Suggestion{
			Label: text[:start] + part,
			Value: part,
			Data:  urlChoice{part, query, start, end},
		})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Label < choices[j].Label })
	return choices
}

func (c *completion) wrapURL(a *App, theme t.ThemeData, input t.TextInput) t.Widget {
	tree, variables := a.collectionURLs(), a.variableChoices()
	// The renderer fills its available width to highlight whole rows and align
	// descriptions. Size that space from what we display, not the full URL
	// labels used for matching, or the flex rows expand to the screen width.
	popupWidth := 0
	for _, s := range tree.suggestions(textOf(input.State), input.State.CursorIndex.Get(), variables) {
		width := ansi.StringWidth(s.Data.(urlChoice).label) + 2 // row padding
		if s.Description != "" {
			width += 2 + ansi.StringWidth(s.Description)
		}
		popupWidth = max(popupWidth, width)
	}
	return urlAutocomplete{Autocomplete: t.Autocomplete{
		State: c.state, Child: input, Width: t.Flex(1),
		MatchMode: t.FilterFuzzy, MaxVisible: 8,
		DismissWhenEmpty: true, DisableKeysWhenHidden: true,
		PopupWidth: t.Cells(popupWidth), PopupStyle: t.Style{BackgroundColor: theme.Surface2},
		OnQueryChange: func(string) {
			text, cursor := input.State.GetText(), input.State.CursorIndex.Peek()
			choices := tree.suggestions(text, cursor, variables)
			var key strings.Builder
			key.WriteString(text + "\x00" + strconv.Itoa(cursor))
			for _, choice := range choices {
				key.WriteString("\x00" + choice.Label + "\x00" + choice.Description)
			}
			if next := key.String(); next != c.key {
				c.key = next
				c.state.SetSuggestions(choices)
			}
		},
		Insert: func(text string, _ int, s t.Suggestion, _ int) (string, int) {
			choice := s.Data.(urlChoice)
			prefix := text[:choice.start] + s.Value
			return prefix + text[choice.end:], uniseg.GraphemeClusterCount(prefix)
		},
		RenderSuggestion: func(s t.Suggestion, active bool, _ t.MatchResult, ctx t.BuildContext) t.Widget {
			choice := s.Data.(urlChoice)
			s.Label = choice.label
			match := t.MatchString(s.Label, choice.query, t.FilterOptions{Mode: t.FilterFuzzy})
			return renderSuggestion(s, active, match, ctx)
		},
	}}
}

// Terma 0.20's autocomplete binds Tab but its TextInput doesn't capture it,
// so the focus manager navigates before consulting that binding. Preserve
// the built input's handlers and capture Tab only while its popup is visible.
type urlAutocomplete struct{ t.Autocomplete }

func (a urlAutocomplete) Build(ctx t.BuildContext) t.Widget {
	column := a.Autocomplete.Build(ctx).(t.Column)
	column.Children[0] = urlCompletionInput{
		TextInput: column.Children[0].(t.TextInput),
		complete:  a.State.Visible.Get(),
	}
	return column
}

type urlCompletionInput struct {
	t.TextInput
	complete bool
}

func (i urlCompletionInput) Build(t.BuildContext) t.Widget { return i }
func (i urlCompletionInput) CapturesKey(key string) bool {
	return key == "tab" && i.complete || i.TextInput.CapturesKey(key)
}
