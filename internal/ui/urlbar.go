package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
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

// sessionTabs is the strip of open requests above the URL bar.
type sessionTabs struct {
	fillWidth
	app *App
}

func (st sessionTabs) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := st.app
	active := a.active.Get()
	sessions := a.sessions.Get()
	children := make([]t.Widget, 0, len(sessions)+1)
	for _, s := range sessions {
		children = append(children, sessionChip{app: a, session: s, active: s.id == active, closable: len(sessions) > 1 || !s.isPristine()})
	}
	children = append(children, t.Text{
		Content: " + ",
		Style:   t.Style{ForegroundColor: theme.TextMuted},
		Click:   func(t.MouseEvent) { a.newTab() },
	})
	return t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: children}
}

type sessionChip struct {
	app      *App
	session  *Session
	active   bool
	closable bool
}

// Jump switches to this request tab, for jump mode (see t.Jumpable).
func (c sessionChip) Jump() { c.app.active.Set(c.session.id) }

func (c sessionChip) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := c.session
	method := s.method.Get()
	title := truncate(s.title.Get(), 28)
	bg := theme.Background
	fg := theme.TextMuted
	if c.active {
		bg = theme.Surface
		fg = theme.Text
	}
	spans := []t.Span{
		{Text: " " + method.Short() + " ", Style: t.SpanStyle{Foreground: methodColor(theme, method), Background: bg, Bold: true}},
		{Text: title, Style: t.SpanStyle{Foreground: fg, Background: bg, Bold: c.active}},
	}
	if s.phase.Get() == exchangeSending {
		spans = append(spans, t.Span{Text: " …", Style: t.SpanStyle{Foreground: theme.AccentText, Background: bg}})
	}
	marker := " "
	if s.dirty.Get() {
		marker = " ●"
	}
	spans = append(spans, t.Span{Text: marker, Style: t.SpanStyle{Foreground: theme.WarningText, Background: bg}})
	id := s.id
	children := []t.Widget{t.Text{Spans: spans, Click: func(t.MouseEvent) { c.app.active.Set(id) }}}
	if c.closable {
		children = append(children, t.Text{
			Content: " ✕ ",
			Style:   t.Style{ForegroundColor: theme.TextMuted, BackgroundColor: bg},
			Click:   func(t.MouseEvent) { c.app.closeSession(id) },
		})
	} else {
		children = append(children, t.Text{Content: " ", Style: t.Style{BackgroundColor: bg}})
	}
	return t.Row{Children: children}
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
					methodSelector{app: a, method: s.method.Get()},
					t.Row{
						Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)},
						Children: []t.Widget{
							t.Text{Content: "▎", Style: t.Style{ForegroundColor: barColor, BackgroundColor: theme.Surface}},
							s.urlVars.wrap(theme, t.TextInput{
								ID:          urlInputID,
								State:       s.url,
								Placeholder: "Enter a URL or paste a curl command…",
								Highlighter: urlHighlighter(theme, resolve),
								Style:       inputStyle(theme, false),
								OnChange:    func(string) { s.urlEdited() },
								OnSubmit:    a.submitURL,
								ExtraKeybinds: []t.Keybind{
									{Key: "down", Name: "Request", Action: func() { t.RequestFocus(requestTabsID) }, Hidden: true},
									{Key: "ctrl+y", Name: "Copy URL", Action: func() {
										t.SetClipboard('c', s.url.GetText())
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
			OnSelect: func(item t.MenuItem) {
				a.setMethod(model.Method(item.Label))
				a.closeMethodMenu()
			},
			OnDismiss: a.closeMethodMenu,
		},
		app: a,
	})
}

// methodMenu is the method dropdown. A MenuItem's Shortcut is only a hint, and
// the selector's letter keys don't reach the menu while it has focus, so the
// menu binds them itself. They go ahead of the menu's own keys so that h picks
// HEAD rather than closing the menu.
type methodMenu struct {
	t.Menu
	app *App
}

func (m methodMenu) Keybinds() []t.Keybind {
	binds := make([]t.Keybind, 0, len(model.Methods))
	for _, method := range model.Methods {
		method := method
		binds = append(binds, t.Keybind{Key: methodHotkeys[method], Name: string(method), Action: func() {
			m.app.setMethod(method)
			m.app.closeMethodMenu()
		}, Hidden: true})
	}
	return append(binds, m.Menu.Keybinds()...)
}

// methodSelector shows the current method. Letter keys switch method
// directly; enter opens a menu.
type methodSelector struct {
	app    *App
	method model.Method
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
	for _, method := range model.Methods {
		method := method
		binds = append(binds, t.Keybind{Key: methodHotkeys[method], Name: string(method), Action: func() { m.app.setMethod(method) }, Hidden: true})
	}
	return binds
}

func (m methodSelector) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	fg := methodColor(theme, m.method)
	bg := theme.Surface
	if ctx.IsFocused(m) {
		bg = theme.Surface3
	}
	return t.Text{
		Spans: []t.Span{
			{Text: " " + padRight(string(m.method), 7), Style: t.SpanStyle{Foreground: fg, Background: bg, Bold: true}},
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
	resp := s.response.Get()
	if resp == nil {
		return t.EmptyWidget{}
	}
	fg, bg := statusColors(theme, resp.StatusCode)
	return t.Text{Content: " " + strconv.Itoa(resp.StatusCode) + " ", Style: t.Style{ForegroundColor: fg, BackgroundColor: bg, Bold: true}}
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
	values := p.app.variableValues()
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
