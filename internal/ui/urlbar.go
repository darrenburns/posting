package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/curl"
	"github.com/darrenburns/posting/v3/internal/model"
)

const (
	urlInputID       = "url-input"
	methodSelectorID = "url-method"
	methodMenuID     = "url-method-menu"
	sendButtonID     = "url-send"
)

var methodHotkeys = map[model.Method]string{
	model.MethodGet:     "g",
	model.MethodPost:    "p",
	model.MethodPut:     "u",
	model.MethodPatch:   "a",
	model.MethodDelete:  "d",
	model.MethodHead:    "h",
	model.MethodOptions: "o",
}

// urlBar is the method selector, URL input, status and send button, with a
// preview line underneath.
type urlBar struct {
	fillWidth
	app     *App
	session *Session
}

func (u urlBar) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a, s := u.app, u.session
	errored := s.phase.Get() == exchangeFailed
	barColor := theme.Surface
	if errored {
		barColor = theme.Error
	}
	resolve := a.resolver()
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsTRBL(a.gap(), 1, 0, 0)},
		Children: []t.Widget{
			t.Row{
				Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1)},
				Spacing: 1,
				Children: []t.Widget{
					methodSelector{app: a, request: s.badgeRequest()},
					t.Row{
						Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)},
						Children: []t.Widget{
							t.Text{Content: "▎", Style: t.Style{ForegroundColor: barColor, BackgroundColor: theme.Surface}},
							s.urlVars.wrap(theme, t.TextInput{
								ID:          urlInputID,
								State:       s.url,
								Placeholder: kindViews[s.kind.Get()].urlPlaceholder,
								Highlighter: urlHighlighter(theme, resolve),
								Style:       inputStyle(theme, false),
								OnChange:    func(string) { s.urlEdited() },
								OnSubmit:    a.submitURL,
								OnPaste:     a.pasteURL,
								ExtraKeybinds: []t.Keybind{
									{Key: "down", Name: "Request", Action: func() { t.RequestFocus(requestTabsID) }, Hidden: true},
									{Key: "ctrl+y", Name: "Copy URL", Action: func() {
										t.SetClipboard(t.SystemClipboard, s.url.GetText())
										a.notify("Copied URL", toastSuccess)
									}, Hidden: true},
								},
							}, a.variableChoices(), t.Flex(1)),
						},
					},
					statusChip{session: s},
					traceMarkers{session: s},
					t.Button{
						ID:           sendButtonID,
						DisableFocus: true,
						Label:        a.icons.send + "Send",
						Variant:      t.ButtonPrimary,
						OnPress:      a.send,
						Click:        func(t.MouseEvent) { a.send() },
					},
				},
			},
			urlPreview{app: a, session: s},
			a.methodMenuWidget(),
		},
	}
}

func (a *App) methodMenuWidget() t.Widget {
	return t.ShowWhen(a.menuOpen.Get(), methodMenu{
		Menu: t.Menu{
			ID:       methodMenuID,
			State:    a.methodMenu,
			AnchorID: methodSelectorID,
			// Terma runs OnSelect in place of the item's Action.
			OnSelect: func(item t.MenuItem) {
				item.Action()
				a.closeMethodMenu()
			},
			OnDismiss: a.closeMethodMenu,
		},
		app: a,
	})
}

// methodMenu is the method and kind dropdown. A MenuItem's Shortcut is only a
// hint, and the selector's letter keys don't reach the menu while it has
// focus, so the menu binds them itself. They go ahead of the menu's own keys
// so that h picks HEAD rather than closing the menu.
type methodMenu struct {
	t.Menu
	app *App
}

func (m methodMenu) Keybinds() []t.Keybind {
	binds := m.app.methodHotkeyBinds()
	for i := range binds {
		choose := binds[i].Action
		binds[i].Action = func() {
			choose()
			m.app.closeMethodMenu()
		}
	}
	return append(binds, m.Menu.Keybinds()...)
}

// methodHotkeyBinds pick each method choice by its letter.
func (a *App) methodHotkeyBinds() []t.Keybind {
	choices := a.methodChoices()
	binds := make([]t.Keybind, len(choices))
	for i, choice := range choices {
		binds[i] = t.Keybind{Key: choice.hotkey, Name: choice.label, Action: choice.choose, Hidden: true}
	}
	return binds
}

// methodSelector shows the request's method, or its kind when it isn't
// HTTP. Letter keys choose directly; enter opens a menu.
type methodSelector struct {
	app     *App
	request model.Request
}

func (m methodSelector) WidgetID() string            { return methodSelectorID }
func (m methodSelector) IsFocusable() bool           { return true }
func (m methodSelector) OnKey(event t.KeyEvent) bool { return false }

// OnClick opens the method menu. Clicks go to the widget as written, not to
// the Text it builds, so a Click set on that Text would never fire.
func (m methodSelector) OnClick(t.MouseEvent) { m.app.openMethodMenu() }

func (m methodSelector) Keybinds() []t.Keybind {
	binds := []t.Keybind{
		{Key: "enter", Name: "Choose method", Action: m.app.openMethodMenu},
		{Key: " ", Name: "Choose method", Action: m.app.openMethodMenu, Hidden: true},
	}
	return append(binds, m.app.methodHotkeyBinds()...)
}

func (m methodSelector) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	fg := requestColor(theme, m.request)
	bg := theme.Surface
	if ctx.IsFocused(m) {
		bg = theme.Surface3
	}
	return t.Text{
		Spans: []t.Span{
			{Text: " " + padRight(requestLabel(m.request), 7), Style: t.SpanStyle{Foreground: fg, Background: bg, Bold: true}},
			{Text: "▾ ", Style: t.SpanStyle{Foreground: theme.TextMuted, Background: bg}},
		},
	}
}

// statusChip shows the status of the last response, or a spinner while sending.
type statusChip struct{ session *Session }

func (c statusChip) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := c.session
	switch s.phase.Get() {
	case exchangeSending:
		return t.Spinner{State: s.spinner, Style: t.Style{ForegroundColor: theme.AccentText, Padding: t.EdgeInsetsXY(1, 0)}}
	case exchangeFailed:
		return t.Text{Content: " ERR ", Style: t.Style{ForegroundColor: theme.ErrorText, BackgroundColor: theme.ErrorBg, Bold: true}}
	}
	if s.response.Get() == nil {
		return t.EmptyWidget{}
	}
	status := s.responseStatus
	fg, bg := statusColors(theme, status.Class)
	return t.Text{Content: " " + status.Code + " ", Style: t.Style{ForegroundColor: fg, BackgroundColor: bg, Bold: true}}
}

// traceMarkers is a compact progress indicator: one square per exchange stage.
type traceMarkers struct{ session *Session }

func (m traceMarkers) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	events := m.session.trace.Get()
	if len(events) == 0 {
		return t.EmptyWidget{}
	}
	states := map[model.TraceStage]model.TraceState{}
	for _, e := range events {
		states[e.Stage] = e.State
	}
	spans := make([]t.Span, 0, len(model.TraceStages))
	for _, stage := range model.TraceStages {
		color := theme.TextDisabled
		switch states[stage] {
		case model.TraceComplete:
			color = theme.Success
		case model.TraceStarted:
			color = theme.Warning
		case model.TraceFailed:
			color = theme.Error
		case model.TraceSkipped:
			color = theme.Border
		}
		spans = append(spans, t.Span{Text: "■", Style: t.SpanStyle{Foreground: color}})
	}
	return t.Text{Spans: spans}
}

// urlPreview explains the URL under the input: the value of the variable at
// the cursor, or the fully resolved URL when variables are present.
type urlPreview struct {
	fillWidth
	app     *App
	session *Session
}

func (p urlPreview) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := p.session
	graphemes := s.url.Content.Get()
	cursor := s.url.CursorIndex.Get()
	text := strings.Join(graphemes, "")
	variables := p.app.resolvedVariables()
	values := model.Values(variables)
	style := t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: t.EdgeInsetsXY(12, 0)}

	if curl.IsCommand(text) {
		hint := "Press [b]enter[/] to import this curl command"
		if strings.HasSuffix(strings.TrimRight(text, " "), "\\") {
			hint = "Press [b]enter[/] to continue the command on the next line"
		}
		preview := t.ParseMarkupToText("[$AccentText]↵[/] [$TextMuted]"+hint+"[/]", theme)
		preview.Style = style
		return preview
	}
	refs := model.FindVariables(text)
	if len(refs) == 0 || !p.app.settings.URLBar.ShowValuePreview {
		return t.Text{Content: "", Style: style}
	}
	cursorByte := 0
	for i := 0; i < cursor && i < len(graphemes); i++ {
		cursorByte += len(graphemes[i])
	}
	for _, ref := range refs {
		if cursorByte < ref.Start || cursorByte > ref.End {
			continue
		}
		value, ok := values[ref.Name]
		spans := []t.Span{{Text: ref.Name, Style: t.SpanStyle{Foreground: theme.AccentText, Bold: true}}, {Text: " = ", Style: t.SpanStyle{Foreground: theme.TextMuted}}}
		switch {
		case !ok:
			spans = append(spans, t.Span{Text: "not set in this environment", Style: t.SpanStyle{Foreground: theme.ErrorText, Italic: true}})
		case model.IsSensitiveName(ref.Name) && p.app.settings.URLBar.HideSecretsInValuePreview:
			spans = append(spans, t.Span{Text: "•••••••• (hidden)", Style: t.SpanStyle{Foreground: theme.TextMuted}})
		default:
			spans = append(spans, t.Span{Text: value, Style: t.SpanStyle{Foreground: theme.Text}})
		}
		if ok {
			i, _ := slices.BinarySearchFunc(variables, ref.Name, func(v model.Variable, name string) int { return strings.Compare(v.Name, name) })
			spans = append(spans, t.Span{Text: "  from " + variableSource(variables[i]), Style: t.SpanStyle{Foreground: theme.TextMuted, Italic: true}})
		}
		return t.Text{Spans: spans, Style: style}
	}
	lookup := func(name string) (string, bool) { v, ok := values[name]; return v, ok }
	resolved := model.Substitute(model.ResolvePathParams(text, s.pathParams.Values()), lookup)
	return t.Text{
		Spans: []t.Span{
			{Text: "→ ", Style: t.SpanStyle{Foreground: theme.TextMuted}},
			{Text: resolved, Style: t.SpanStyle{Foreground: theme.TextMuted, Italic: true}},
		},
		Style: style,
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func padLeft(s string, n int) string {
	width := utf8.RuneCountInString(s)
	if width >= n {
		return s
	}
	return strings.Repeat(" ", n-width) + s
}

func padRight(s string, n int) string {
	width := utf8.RuneCountInString(s)
	if width >= n {
		return s
	}
	return s + strings.Repeat(" ", n-width)
}
