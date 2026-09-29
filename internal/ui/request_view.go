package ui

import (
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const requestTabsID = "req-tabs"

// requestPanel is the tabbed request editor.
type requestPanel struct {
	fillParent
	app     *App
	session *Session
}

func (p requestPanel) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "alt+z", Name: "Expand", Action: func() { p.app.toggleExpand("request") }, Hidden: true},
	}
}

// requestTabs is the request panel's tab strip.
func (s *Session) requestTabs() tabStrip {
	return tabStrip{
		ID:     requestTabsID,
		Active: s.requestTab,
		View:   s.requestTabView,
		Tabs: []tabItem{
			{Key: "headers", Label: "Headers", Badge: countBadge(s.headers.Count())},
			{Key: "body", Label: "Body", Marked: s.bodyType.Get() != model.BodyNone},
			{Key: "path", Label: "Path", Badge: countBadge(s.pathParams.Count())},
			{Key: "query", Label: "Query", Badge: countBadge(s.query.Count())},
			{Key: "auth", Label: "Auth", Marked: s.authType.Get() != model.AuthNone},
			{Key: "info", Label: "Info"},
			{Key: "options", Label: "Options"},
		},
	}
}

func (p requestPanel) Build(ctx t.BuildContext) t.Widget {
	s := p.session
	resolve := p.app.resolver()
	theme := ctx.Theme()
	variables := variableHighlighter(theme, resolve)
	choices := p.app.variableChoices()
	gap := p.app.gap()
	return section{
		Prefix: "req-",
		Title:  "Request",
		Width:  t.Flex(1),
		Height: t.Flex(1),
		Child: t.Column{
			Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Spacing: gap,
			Children: []t.Widget{
				s.requestTabs(),
				t.Switcher{
					Active: s.requestTab.Get(),
					Style:  t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: inset},
					Children: map[string]t.Widget{
						"headers": kvEditorView{
							Editor:           s.headers,
							KeyPlaceholder:   "Add a header…",
							ValuePlaceholder: "Value",
							ValueHighlighter: variables,
							Choices:          choices,
						},
						"body": bodyEditor{app: p.app, session: s},
						"path": kvEditorView{
							Editor:           s.pathParams,
							ValuePlaceholder: "Value",
							ValueHighlighter: variables,
							Choices:          choices,
							Empty: emptyState{
								Title: "No path parameters",
								Lines: []string{"Add [b]:name[/] segments to the URL to create them", "e.g. https://example.com/users/[b $Info]:id[/]"},
							},
						},
						"query": kvEditorView{
							Editor:           s.query,
							KeyPlaceholder:   "Add a parameter…",
							ValuePlaceholder: "Value",
							ValueHighlighter: variables,
							Choices:          choices,
						},
						"auth":    authEditor{session: s, variables: variables, choices: choices, gap: gap},
						"info":    infoEditor{session: s, gap: gap},
						"options": optionsEditor{session: s, variables: variables, choices: choices, gap: gap},
					},
				},
			},
		},
	}
}

// bodyEditor selects the body type and edits its content.
type bodyEditor struct {
	fillParent
	app     *App
	session *Session
}

var contentTypes = []choice{
	{Value: "application/json", Label: "JSON"},
	{Value: "text/plain", Label: "Text"},
	{Value: "application/xml", Label: "XML"},
	{Value: "text/html", Label: "HTML"},
}

func (b bodyEditor) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := b.session
	bodyType := s.bodyType.Get()
	controls := []t.Widget{
		segmented{
			ID: "req-body-type",
			Options: []choice{
				{Value: string(model.BodyNone), Label: "None"},
				{Value: string(model.BodyRaw), Label: "Raw"},
				{Value: string(model.BodyForm), Label: "Form"},
			},
			Selected: string(bodyType),
			OnChange: func(value string) { s.bodyType.Set(model.BodyType(value)); s.touch() },
		},
		t.Spacer{},
	}
	var content t.Widget
	switch bodyType {
	case model.BodyRaw:
		contentType := s.contentType.Get()
		controls = append(controls, segmented{
			ID:       "req-body-content-type",
			Options:  contentTypes,
			Selected: contentType,
			OnChange: func(value string) { s.contentType.Set(value); s.touch() },
		})
		var area t.Widget = t.TextArea{
			ID:          "req-body-text",
			State:       s.body,
			ScrollState: s.bodyScroll,
			Placeholder: "Request body…",
			Highlighter: b.app.bodyHighlighter(theme, languageFor(contentType), s.substitute.Checked.Get()),
			Style:       t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
			OnChange:    func(string) { s.touch() },
		}
		// Variables are only worth completing when they'll be substituted.
		if s.substitute.Checked.Get() {
			area = s.bodyVars.wrap(theme, area, b.app.variableChoices(), t.Flex(1))
		}
		content = scrollingArea("req-body-text", s.bodyScroll, theme.Surface, area)
	case model.BodyForm:
		content = kvEditorView{
			Editor:           s.form,
			KeyPlaceholder:   "Add a field…",
			ValuePlaceholder: "Value",
			ValueHighlighter: variableHighlighter(theme, b.app.resolver()),
		}
		if s.substitute.Checked.Get() {
			form := content.(kvEditorView)
			form.Choices = b.app.variableChoices()
			content = form
		}
	default:
		content = emptyState{Title: "No request body", Lines: []string{"Choose [b]Raw[/] for JSON, XML or text, or [b]Form[/] for URL-encoded fields"}}
	}
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: b.app.gap(),
		Children: []t.Widget{
			t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: controls},
			content,
		},
	}
}

// bodyHighlighter combines syntax and variable highlighting for the request
// body. Highlighters are cached so tokenising survives rebuilds.
func (a *App) bodyHighlighter(theme t.ThemeData, language string, variables bool) t.Highlighter {
	key := theme.Name + "/" + language
	if a.highlighters == nil {
		a.highlighters = map[string]*syntaxHighlighter{}
	}
	syntax, ok := a.highlighters[key]
	if !ok {
		syntax = newSyntaxHighlighter(theme, language)
		a.highlighters[key] = syntax
	}
	if !variables {
		return syntax
	}
	resolve := a.resolver()
	return t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		out := syntax.Highlight(text, graphemes)
		return append(out, variableHighlights(theme, text, byteToGrapheme(graphemes, len(text)), resolve)...)
	})
}

// authEditor chooses an auth scheme and edits its credentials.
type authEditor struct {
	fillParent
	session   *Session
	variables t.Highlighter
	choices   variableChoices
	gap       int
}

func (e authEditor) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := e.session
	authType := s.authType.Get()
	touch := func(string) { s.touch() }
	rows := []formField{
		field(t.Column{Children: []t.Widget{
			segmented{
				ID: "req-auth-type",
				Options: []choice{
					{Value: string(model.AuthNone), Label: "None"},
					{Value: string(model.AuthBasic), Label: "Basic"},
					{Value: string(model.AuthDigest), Label: "Digest"},
					{Value: string(model.AuthBearer), Label: "Bearer token"},
				},
				Selected: string(authType),
				OnChange: func(value string) { s.authType.Set(model.AuthType(value)); s.touch() },
			},
			t.Text{Content: "The Authorization header is generated when the request is sent.", Style: t.Style{ForegroundColor: theme.TextMuted, Margin: t.EdgeInsetsTRBL(e.gap, 0, 0, 0), Padding: inset}},
		}}, "req-auth-type"),
	}
	switch authType {
	case model.AuthBasic, model.AuthDigest:
		rows = append(rows,
			field(formRow(ctx, "Username", "", input{ID: "req-auth-username", State: s.username, Placeholder: "Enter a username", Highlighter: e.variables, OnChange: touch, Completion: s.usernameVars, Choices: e.choices}), "req-auth-username"),
			field(formRow(ctx, "Password", "", input{ID: "req-auth-password", State: s.password, Placeholder: "Enter a password", Highlighter: e.variables, OnChange: touch, Completion: s.passwordVars, Choices: e.choices}), "req-auth-password"),
		)
	case model.AuthBearer:
		hint := ""
		if isBlank(s.token) {
			hint = "required"
		}
		rows = append(rows,
			field(formRow(ctx, "Token", hint, input{ID: "req-auth-token", State: s.token, Placeholder: "Enter a token, e.g. ${API_TOKEN}", Highlighter: e.variables, OnChange: touch, Completion: s.tokenVars, Choices: e.choices}), "req-auth-token"),
		)
	default:
		rows = append(rows, field(emptyState{Title: "No authentication", Lines: []string{"This request is sent without credentials"}}))
	}
	return scrollForm{State: s.authScroll, Spacing: e.gap, Rows: rows}
}

// formLabelWidth fits the longest form label ("Post-response") with room to
// breathe, so fields in every tab start in the same column.
const formLabelWidth = 16

// formLabel is the label column of a form row.
func formLabel(spans []t.Span) t.Widget {
	return t.Text{Spans: spans, Style: t.Style{Width: t.Cells(formLabelWidth), Padding: inset}}
}

// formRow lays out a fixed-width label beside a field.
func formRow(ctx t.BuildContext, label, hint string, field t.Widget) t.Widget {
	theme := ctx.Theme()
	labelSpans := []t.Span{{Text: label, Style: t.SpanStyle{Foreground: theme.Text, Bold: true}}}
	if hint != "" {
		labelSpans = append(labelSpans, t.Span{Text: " " + hint, Style: t.SpanStyle{Foreground: theme.ErrorText, Italic: true}})
	}
	return t.Row{
		Style: t.Style{Width: t.Flex(1)},
		Children: []t.Widget{
			formLabel(labelSpans),
			field,
		},
	}
}

// infoEditor edits the request's name and description.
type infoEditor struct {
	fillParent
	session *Session
	gap     int
}

func (e infoEditor) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := e.session
	file := s.file.Get()
	fileText := t.Text{Content: file, Style: t.Style{ForegroundColor: theme.Text}}
	if file == "" {
		fileText = t.Text{Content: "Not saved yet — press ctrl+s to save it to the collection", Style: t.Style{ForegroundColor: theme.TextMuted, Italic: true}}
	}
	// The description comes last: it takes whatever height is left and
	// scrolls itself, so a short panel squeezes it rather than hiding the
	// fields after it.
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: e.gap,
		Children: []t.Widget{
			formRow(ctx, "Name", "", input{ID: "req-info-name", State: s.name, Placeholder: "Enter a name…", OnChange: func(string) { s.touch() }}),
			t.Row{Children: []t.Widget{
				formLabel([]t.Span{{Text: "File", Style: t.SpanStyle{Foreground: theme.Text, Bold: true}}}),
				fileText,
			}},
			t.Row{
				Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
				Children: []t.Widget{
					formLabel([]t.Span{{Text: "Description", Style: t.SpanStyle{Foreground: theme.Text, Bold: true}}}),
					scrollingArea("req-info-description", s.descriptionScroll, theme.Surface, t.TextArea{
						ID:          "req-info-description",
						State:       s.description,
						ScrollState: s.descriptionScroll,
						Placeholder: "Describe what this request does…",
						Style:       t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
						OnChange:    func(string) { s.touch() },
					}),
				},
			},
		},
	}
}

// optionsEditor edits transport options.
type optionsEditor struct {
	fillParent
	session   *Session
	variables t.Highlighter
	choices   variableChoices
	gap       int
}

func (e optionsEditor) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := e.session
	check := func(id string, state *t.CheckboxState, label, help string) t.Widget {
		return t.Row{Spacing: 2, Style: t.Style{Padding: inset}, Children: []t.Widget{
			&t.Checkbox{
				ID:       id,
				State:    state,
				Label:    label,
				Style:    t.Style{Width: t.Cells(28), BackgroundColor: theme.Background},
				OnChange: func(bool) { s.touch() },
			},
			t.Text{Content: help, Style: t.Style{ForegroundColor: theme.TextMuted}},
		}}
	}
	touch := func(string) { s.touch() }
	// Each checkbox is its own row so tabbing through them scrolls one
	// line at a time; the fields below keep a gap above them.
	gap := func(w t.Widget) t.Widget {
		return t.Column{Style: t.Style{Width: t.Flex(1), Margin: t.EdgeInsetsTRBL(e.gap, 0, 0, 0)}, Children: []t.Widget{w}}
	}
	return scrollForm{
		State: s.optionsScroll,
		Rows: []formField{
			field(check("req-opt-follow", s.follow, "Follow redirects", "Follow 3xx responses to their destination"), "req-opt-follow"),
			field(check("req-opt-verify", s.verifySSL, "Verify SSL certificates", "Reject servers with invalid certificates"), "req-opt-verify"),
			field(check("req-opt-cookies", s.cookies, "Attach cookies", "Send cookies stored from earlier responses"), "req-opt-cookies"),
			field(check("req-opt-substitute", s.substitute, "Substitute body variables", "Replace ${VAR} references in the body"), "req-opt-substitute"),
			field(gap(formRow(ctx, "Proxy URL", "", input{ID: "req-opt-proxy", State: s.proxy, Placeholder: "http://proxy.example.com:8080", Highlighter: e.variables, OnChange: touch, Completion: s.proxyVars, Choices: e.choices})), "req-opt-proxy"),
			field(gap(formRow(ctx, "Timeout", "", input{ID: "req-opt-timeout", State: s.timeout, Placeholder: "seconds", Width: t.Cells(12), OnChange: touch})), "req-opt-timeout"),
		},
	}
}
