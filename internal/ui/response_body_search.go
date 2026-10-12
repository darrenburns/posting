package ui

import (
	"fmt"

	t "github.com/darrenburns/terma"
)

const responseSearchID = "resp-body-search"

func (s *Session) openBodySearch() {
	s.responseVisual.Set(false)
	s.messages.following.Set(false)
	s.responseSearchOpen.Set(true)
	s.responseSearch.SelectAll()
	s.searchBody(s.responseSearch.GetText())
	t.RequestFocus(responseSearchID)
}

func (s *Session) searchBody(query string) {
	s.responseBody.SetSearch(query)
	s.revealBodyCursor()
}

func (s *Session) nextBodyMatch() {
	s.responseVisual.Set(false)
	s.responseBody.NextMatch()
	s.revealBodyCursor()
}

func (s *Session) previousBodyMatch() {
	s.responseVisual.Set(false)
	s.responseBody.PreviousMatch()
	s.revealBodyCursor()
}

func (s *Session) closeBodySearch() {
	s.responseSearchOpen.Set(false)
	// Keep the selected result for copying, and the query for reopening.
	s.responseBody.ClearSearch()
	t.RequestFocus("resp-body")
}

// responseBodySearch uses Terma's literal, case-insensitive text-area search.
// Keeping the count in its own widget avoids rebuilding the body as it changes.
type responseBodySearch struct {
	fillWidth
	session *Session
}

func (r responseBodySearch) Build(ctx t.BuildContext) t.Widget {
	s, theme := r.session, ctx.Theme()
	current, total := s.responseBody.SearchPosition()
	count, color := "", theme.TextMuted
	if s.responseBody.SearchQuery.Get() != "" {
		count = fmt.Sprintf("%d/%d", current, total)
		if total == 0 {
			count, color = "No matches", theme.WarningText
		}
	}
	return t.Row{
		Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
		Spacing: 1,
		Children: []t.Widget{
			t.Text{Content: "Find", Style: t.Style{ForegroundColor: theme.AccentText}},
			t.TextInput{
				ID:          responseSearchID,
				State:       s.responseSearch,
				Placeholder: "Find in response…",
				Style:       t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface},
				OnChange:    s.searchBody,
				ExtraKeybinds: []t.Keybind{
					{Key: "enter", Name: "Next match", Action: s.nextBodyMatch},
					{Key: "shift+enter", Name: "Previous match", Action: s.previousBodyMatch},
					{Key: "escape", Name: "Close search", Action: s.closeBodySearch},
					{Key: "up", Name: "Body", Action: func() { t.RequestFocus("resp-body") }},
					{Key: "ctrl+f", Action: s.openBodySearch, Hidden: true},
				},
			},
			t.Text{ID: responseSearchID + "-count", Content: count, Style: t.Style{ForegroundColor: color}},
			t.Text{Content: "↑", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { s.previousBodyMatch() }},
			t.Text{Content: "↓", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { s.nextBodyMatch() }},
			t.Text{Content: "×", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { s.closeBodySearch() }},
		},
	}
}
