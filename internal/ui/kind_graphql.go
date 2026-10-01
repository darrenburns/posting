package ui

import (
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const (
	graphQLQueryID     = "req-gql-query"
	graphQLOperationID = "req-gql-operation"
	graphQLVariablesID = "req-gql-variables"
)

// graphQLEditor edits a GraphQL payload: the document, the operation to run
// and the variables JSON.
type graphQLEditor struct {
	session         *Session
	query           *t.TextAreaState
	queryScroll     *t.ScrollState
	operation       *t.TextInputState
	variables       *t.TextAreaState
	variablesScroll *t.ScrollState
	variablesVars   *completion
}

func newGraphQLEditor(s *Session) payloadEditor {
	return &graphQLEditor{
		session:         s,
		query:           t.NewTextAreaState(""),
		queryScroll:     t.NewScrollState(),
		operation:       t.NewTextInputState(""),
		variables:       t.NewTextAreaState(""),
		variablesScroll: t.NewScrollState(),
		variablesVars:   newCompletion(),
	}
}

func (e *graphQLEditor) load(req model.Request) {
	g, _ := req.Payload.(model.GraphQL)
	e.query.SetText(g.Query)
	e.operation.SetText(g.OperationName)
	e.variables.SetText(g.Variables)
	e.query.CursorIndex.Set(0)
	e.variables.CursorIndex.Set(0)
	e.queryScroll.SetOffset(0)
	e.variablesScroll.SetOffset(0)
}

func (e *graphQLEditor) payload() model.Payload {
	return model.GraphQL{
		Query:         e.query.GetText(),
		Variables:     e.variables.GetText(),
		OperationName: strings.TrimSpace(e.operation.GetText()),
	}
}

func (e *graphQLEditor) tabs() []requestTab {
	return []requestTab{
		{key: "gql-query", label: "Query", jump: "i", marked: func() bool { return hasText(e.query) }},
		{key: "gql-variables", label: "Variables", jump: "o", marked: func() bool { return hasText(e.variables) }},
	}
}

func (e *graphQLEditor) view(tab string, a *App) t.Widget {
	if tab == "gql-variables" {
		return graphQLVariablesView{app: a, editor: e}
	}
	return graphQLQueryView{app: a, editor: e}
}

func (e *graphQLEditor) focusID(tab string) string {
	switch tab {
	case "gql-query":
		return graphQLQueryID
	case "gql-variables":
		return graphQLVariablesID
	}
	return ""
}

// hasText reports reactively whether a text area holds more than whitespace.
func hasText(area *t.TextAreaState) bool {
	return t.SelectAny(area.Content, func(graphemes []string) bool {
		return strings.TrimSpace(strings.Join(graphemes, "")) != ""
	})
}

// graphQLQueryView is the Query tab: the operation name above the document.
type graphQLQueryView struct {
	fillParent
	app    *App
	editor *graphQLEditor
}

func (v graphQLQueryView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, s := v.editor, v.editor.session
	touch := func(string) { s.touch() }
	// Query isn't offered variable completion: typing $ there starts a
	// GraphQL variable far more often than a Posting one.
	area := t.TextArea{
		ID:            graphQLQueryID,
		State:         e.query,
		ScrollState:   e.queryScroll,
		Placeholder:   "Enter a GraphQL query…",
		Highlighter:   v.app.queryHighlighter(theme, s.substitute.Checked.Get()),
		Style:         t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
		OnChange:      touch,
		ExtraKeybinds: v.app.externalKeybinds(e.query, "graphql", s.touch),
	}
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: v.app.gap(),
		Children: []t.Widget{
			formRow(ctx, "Operation", "", input{ID: graphQLOperationID, State: e.operation, Placeholder: "Operation to run when the query defines several", OnChange: touch}),
			scrollingArea(graphQLQueryID, e.queryScroll, theme.Surface, area),
		},
	}
}

// graphQLVariablesView is the Variables tab: a JSON object, substituted and
// completed like a raw JSON body.
type graphQLVariablesView struct {
	fillParent
	app    *App
	editor *graphQLEditor
}

func (v graphQLVariablesView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, s := v.editor, v.editor.session
	substitute := s.substitute.Checked.Get()
	var area t.Widget = t.TextArea{
		ID:            graphQLVariablesID,
		State:         e.variables,
		ScrollState:   e.variablesScroll,
		Placeholder:   `{"id": "${USER_ID}"}`,
		Highlighter:   v.app.bodyHighlighter(theme, "json", substitute),
		Style:         t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
		OnChange:      func(string) { s.touch() },
		ExtraKeybinds: v.app.externalKeybinds(e.variables, "json", s.touch),
	}
	if substitute {
		area = e.variablesVars.wrap(theme, area, v.app.variableChoices(), t.Flex(1))
	}
	return scrollingArea(graphQLVariablesID, e.variablesScroll, theme.Surface, area)
}

// queryHighlighter highlights a GraphQL document, and the ${NAME}
// references in it when they will be substituted. A bare $name is a GraphQL
// variable and is left to the syntax colours.
func (a *App) queryHighlighter(theme t.ThemeData, variables bool) t.Highlighter {
	syntax := a.syntax(theme, "graphql")
	if !variables {
		return syntax
	}
	resolve := a.resolver()
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		out := syntax.Highlight(text, graphemes)
		return append(out, refHighlights(theme, model.FindBracedVariables(text), byteToGrapheme(graphemes, len(text)), resolve)...)
	})
}
