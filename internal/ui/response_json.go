package ui

import (
	"encoding/json"
	"fmt"

	t "github.com/darrenburns/terma"
)

const responseJSONQueryID = "resp-jsonpath"

// A new body starts with an unfiltered tree. Invalid JSON must discard the
// old tree rather than leave another response's data on screen. Detect JSON
// from the body, including when the server omitted or mislabelled its type.
func (s *Session) setResponseJSON(body []byte) {
	state, _ := t.NewJSONViewerState(body)
	s.responseJSON.Set(state)
	s.responseJSONQuery.SetText("")
	s.responseJSONError.Set("")
	if state == nil {
		s.responseJSONTree.Set(false)
	}
}

func (s *Session) toggleBodyView() {
	s.responseJSONTree.Set(!s.responseJSONTree.Peek())
	s.messages.following.Set(false)
	t.RequestFocus("resp-body")
}

// responseJSON is the optional tree view of the same body used by the text
// viewer and external tools. Terma owns parsing, filtering and navigation.
type responseJSON struct {
	fillParent
	app     *App
	session *Session
	state   *t.JSONViewerState
}

func (r responseJSON) focusQuery() {
	r.session.responseJSONQuery.SelectAll()
	t.RequestFocus(responseJSONQueryID)
}

func (r responseJSON) setQuery(query string) {
	message := ""
	if err := r.state.SetQuery(query); err != nil {
		message = "Invalid JSONPath · showing last valid results"
	}
	r.session.responseJSONError.Set(message)
}

func (r responseJSON) clearQuery() {
	r.session.responseJSONQuery.SetText("")
	r.setQuery("")
	t.RequestFocus("resp-body")
}

func (r responseJSON) copy(path bool) {
	node, ok := r.state.Tree.CursorNode()
	if !ok {
		return
	}
	text, message := node.Path, "Copied JSONPath"
	if !path {
		data, _ := json.MarshalIndent(node.Value, "", "  ")
		text, message = string(data), "Copied JSON value"
	}
	t.SetClipboard(t.SystemClipboard, text)
	r.app.notify(message, toastSuccess)
}

func (r responseJSON) Keybinds() []t.Keybind {
	binds := []t.Keybind{
		{Key: "ctrl+f", Name: "JSONPath", Action: r.focusQuery},
		{Key: "/", Action: r.focusQuery, Hidden: true},
		{Key: "y", Name: "Copy value", Action: func() { r.copy(false) }},
		{Key: "c", Action: func() { r.copy(false) }, Hidden: true},
		{Key: "Y", Name: "Copy path", Action: func() { r.copy(true) }},
		{Key: "e", Name: "Expand all", Action: r.state.Tree.ExpandAll},
		{Key: "E", Name: "Collapse all", Action: r.state.Tree.CollapseAll},
	}
	if r.session.responseJSONQuery.GetText() != "" {
		binds = append(binds, t.Keybind{Key: "escape", Name: "Clear filter", Action: r.clearQuery})
	}
	return append(binds, r.app.externalKeybinds(r.session.responseBody, "json", nil)...)
}

func (r responseJSON) Build(ctx t.BuildContext) t.Widget {
	s, theme := r.session, ctx.Theme()
	count := ""
	color := theme.TextMuted
	if r.state.Query() != "" {
		count = fmt.Sprintf("%d matches", r.state.MatchCount())
		if r.state.MatchCount() == 1 {
			count = "1 match"
		}
		if r.state.MatchCount() == 0 {
			color = theme.WarningText
		}
	}
	query := t.Row{
		Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: inset},
		Spacing: 1,
		Children: []t.Widget{
			t.Text{Content: "JSONPath", Style: t.Style{ForegroundColor: theme.AccentText}},
			t.TextInput{
				ID: responseJSONQueryID, State: s.responseJSONQuery,
				Placeholder: "e.g. $.users[*]", Style: t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface},
				OnChange:      r.setQuery,
				OnSubmit:      func(string) { t.RequestFocus("resp-body") },
				ExtraKeybinds: []t.Keybind{{Key: "escape", Name: "Clear filter", Action: r.clearQuery}},
			},
			t.Text{ID: responseJSONQueryID + "-count", Content: count, Style: t.Style{ForegroundColor: color}},
			t.Text{ID: responseJSONQueryID + "-clear", Content: "×", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { r.clearQuery() }},
		},
	}
	children := []t.Widget{
		t.JSONViewer{ID: "resp-body", State: r.state, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}},
	}
	if message := s.responseJSONError.Get(); message != "" {
		children = append(children, t.Text{
			ID: responseJSONQueryID + "-error", Content: message,
			Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), ForegroundColor: theme.ErrorText, Padding: inset},
		})
	}
	children = append(children,
		query,
		responseJSONStatus{view: r},
	)
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: children}
}

// Only the status row needs to rebuild when the cursor's path changes.
type responseJSONStatus struct {
	fillWidth
	view responseJSON
}

func (status responseJSONStatus) Build(ctx t.BuildContext) t.Widget {
	r := status.view
	theme := ctx.Theme()
	r.state.Tree.CursorPath.Get()
	node, _ := r.state.Tree.CursorNode()
	return t.Row{
		Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: inset},
		Spacing: 2,
		Children: []t.Widget{
			t.Text{ID: "resp-json-path", Content: node.Path, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}},
			t.Text{ID: "resp-body-text", Content: "Text view", Style: t.Style{ForegroundColor: theme.AccentText}, Click: func(t.MouseEvent) { r.session.toggleBodyView() }},
			t.Text{Content: "copy value", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { r.copy(false) }},
		},
	}
}
