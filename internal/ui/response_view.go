package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const responseTabsID = "resp-tabs"

// responsePanel shows the latest response for the session.
type responsePanel struct {
	fillParent
	app     *App
	session *Session
}

func (p responsePanel) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "alt+z", Name: "Expand", Action: func() { p.app.toggleExpand("response") }, Hidden: true},
	}
}

func (p responsePanel) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s := p.session
	resp := s.response.Get()
	phase := s.phase.Get()
	streaming := s.streaming.Get()
	live := phase == exchangeSending && s.live

	title, subtitle := "", ""
	switch {
	case live:
		label := "RECEIVING"
		if streaming == streamOpen {
			label = "STREAM OPEN"
		}
		fg, bg := statusColors(theme, model.StatusClassWarning)
		title = fmt.Sprintf(" [b %s on %s] %s [/]", fg.Hex(), bg.Hex(), label)
		subtitle = p.streamHints(streaming)
	case resp != nil:
		status := s.responseStatus
		fg, bg := statusColors(theme, status.Class)
		title = fmt.Sprintf(" [b %s on %s] %s [/]", fg.Hex(), bg.Hex(), statusText(status))
		if p.app.settings.Response.ShowSizeAndTime {
			subtitle = fmt.Sprintf("[$Text]%s[/] [$TextMuted]in[/] [$TextMuted]%s[/][$Text]%s[/]", model.FormatBytes(resp.Size()), p.app.icons.elapsed, model.FormatDuration(resp.Elapsed))
		}
	}

	var content t.Widget
	switch {
	case phase == exchangeFailed:
		err := s.err.Get()
		message := "unknown error"
		if err != nil {
			message = err.Error()
		}
		content = emptyState{Title: "Request failed", Lines: []string{"[$Error]" + escapeMarkup(message) + "[/]", "Press [b]ctrl+j[/] to try again"}}
	case phase == exchangeSending && !s.live && streaming != streamNone:
		label := "Opening stream…"
		if streaming != streamStarting {
			label = "Waiting for the server…"
		}
		content = waiting(theme, s.spinner, label, p.streamHints(streaming))
	case resp == nil && phase == exchangeSending:
		content = waiting(theme, s.spinner, "Sending request…", "[$TextMuted]Press [b]esc[/] to cancel[/]")
	case resp == nil:
		content = emptyState{Title: "No response yet", Lines: []string{"Press [b]ctrl+j[/] to send the request"}}
	default:
		content = responseTabs{app: p.app, session: s, response: resp}
	}

	return section{
		Prefix:    "resp-",
		Title:     "Response",
		TitleMark: title,
		Subtitle:  subtitle,
		Width:     t.Flex(1),
		Height:    t.Flex(1),
		Child:     content,
	}
}

// waiting is a spinner and label above a line of hints, in markup.
func waiting(theme t.ThemeData, spinner *t.SpinnerState, label, hints string) t.Widget {
	return t.Column{
		Style:      t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		MainAlign:  t.MainAxisCenter,
		CrossAlign: t.CrossAxisCenter,
		Children: []t.Widget{
			t.Row{Spacing: 1, Children: []t.Widget{
				t.Spinner{State: spinner, Style: t.Style{ForegroundColor: theme.AccentText}},
				t.Text{Content: label, Style: t.Style{ForegroundColor: theme.Text}},
			}},
			t.ParseMarkupToText(hints, theme),
		},
	}
}

// streamHints are the keys for the exchange in flight: sending into and
// ending a stream that is open, and stopping the call.
func (p responsePanel) streamHints(streaming streamPhase) string {
	hint := func(key, what string) string {
		return "[$TextMuted][b]" + escapeMarkup(key) + "[/] " + what + "[/]"
	}
	var hints []string
	if streaming == streamOpen {
		hints = append(hints, hint(p.app.keyHint("send-request"), "sends the message"), hint(p.app.keyHint("stream"), "ends the stream"))
	}
	hints = append(hints, hint("esc", "stops the call"))
	return strings.Join(hints, "[$TextMuted] · [/]")
}

type responseTabs struct {
	fillParent
	app      *App
	session  *Session
	response *model.Response
}

// statusText is a status as the response title and history show it:
// "200 OK", "NOT_FOUND no book 42", or just "OK" for a gRPC success.
func statusText(status model.Status) string {
	return strings.TrimSpace(status.Code + " " + status.Text)
}

// responseTabKeys are the response tabs for a response to req.
func responseTabKeys(req model.Request) []string {
	return kindViews[req.Kind().ID].responseTabs
}

// responseTabs is the response panel's tab strip for resp, the response to
// the request the session sent.
func (s *Session) responseTabs(resp *model.Response) tabStrip {
	tabs := map[string]struct {
		item    tabItem
		focusID string
		empty   bool
	}{
		"body":     {tabItem{Key: "body", Label: "Body"}, "resp-body", len(resp.Body) == 0},
		"headers":  {tabItem{Key: "headers", Label: "Headers", Badge: countBadge(len(resp.Headers))}, "resp-headers", len(resp.Headers) == 0},
		"cookies":  {tabItem{Key: "cookies", Label: "Cookies", Badge: countBadge(len(resp.Cookies))}, "resp-cookies", len(resp.Cookies) == 0},
		"trailers": {tabItem{Key: "trailers", Label: "Trailers", Badge: countBadge(len(resp.Trailers))}, "resp-trailers", len(resp.Trailers) == 0},
		"trace":    {tabItem{Key: "trace", Label: "Trace"}, "resp-trace-scroll", false},
	}
	var items []tabItem
	for _, key := range responseTabKeys(s.sent) {
		items = append(items, tabs[key].item)
	}
	return tabStrip{
		ID:     responseTabsID,
		Active: s.responseTab,
		View:   s.responseTabView,
		Down: func() {
			if tab := tabs[s.responseTab.Peek()]; !tab.empty {
				t.RequestFocus(tab.focusID)
			}
		},
		Up:   func() { t.RequestFocus(requestTabsID) },
		Tabs: items,
	}
}

func (r responseTabs) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s, resp := r.session, r.response
	history := s.fromHistory.Get()
	children := []t.Widget{}
	if history != nil {
		// The URL bar above already shows what was sent.
		children = append(children, t.Text{
			Spans: []t.Span{
				{Text: " HISTORY ", Style: t.SpanStyle{Foreground: theme.TextOnAccent, Background: theme.Accent, Bold: true}},
				{Text: " sent " + history.SentAt.Format("02 Jan 2006 15:04:05"), Style: t.SpanStyle{Foreground: theme.TextMuted}},
			},
			Style: t.Style{Padding: inset},
		})
	}
	children = append(children,
		s.responseTabs(resp),
		t.Switcher{
			Active: s.responseTab.Get(),
			Style:  t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: inset},
			Children: map[string]t.Widget{
				"body":     responseBody{app: r.app, session: s, response: resp},
				"headers":  responseHeaders{id: "resp-headers", state: s.responseHeaders, scroll: s.responseHeadersScroll, empty: "No headers"},
				"cookies":  responseCookies{session: s},
				"trailers": responseHeaders{id: "resp-trailers", state: s.responseTrailers, scroll: s.responseTrailersScroll, empty: "No trailers"},
				"trace":    responseTrace{session: s},
			},
		},
	)
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Spacing: r.app.gap(), Children: children}
}

// responseBody is a read-only, highlighted view of the body.
type responseBody struct {
	fillParent
	app      *App
	session  *Session
	response *model.Response
}

func (b responseBody) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	s, resp := b.session, b.response
	if len(resp.Body) == 0 {
		return emptyState{Title: "Empty body", Lines: []string{"The server returned " + statusText(s.responseStatus) + " with no content"}}
	}
	contentType := resp.ContentType()
	copyBody := func() { b.app.notify(s.copyBody(), toastSuccess) }
	highlighter := b.app.bodyHighlighter(theme, languageFor(contentType), false)
	if isFocusedID(ctx, "resp-body") {
		highlighter = withBracketMatch(highlighter, s.responseBody)
	}
	highlighter = s.withBodyVisualSelection(highlighter, theme.Selection)
	area := t.TextArea{
		ID:            "resp-body",
		State:         s.responseBody,
		ScrollState:   s.responseBodyScroll,
		Highlighter:   highlighter,
		Style:         t.Style{Width: t.Flex(1), BackgroundColor: theme.Background, Padding: t.EdgeInsetsXY(1, 0)},
		MouseDown:     func(t.MouseEvent) { s.responseVisual.Set(false) },
		ExtraKeybinds: append(s.responseBodyKeybinds(copyBody, s.responseVisual.Get()), b.app.externalKeybinds(s.responseBody, languageFor(contentType), nil)...),
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{
			responseBodyViewport{
				height: &s.responseBodyViewportHeight,
				Column: t.Column{
					Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1)},
					Children: []t.Widget{scrollingArea("resp-body", s.responseBodyScroll, theme.Background, area)},
				},
			},
			responseBodyStatus{session: s, contentType: contentType, copyBody: copyBody},
		},
	}
}

type responseBodyViewport struct {
	t.Column
	height *int
}

func (v responseBodyViewport) Build(t.BuildContext) t.Widget { return v }

func (v responseBodyViewport) ChildWidgets() []t.Widget { return v.Children }

func (v responseBodyViewport) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	*v.height = metrics.Box().ContentHeight()
}

// responseBodyStatus is the bar under the response body. It rebuilds as the
// cursor moves, so the body above doesn't have to.
type responseBodyStatus struct {
	fillWidth
	session     *Session
	contentType string
	copyBody    func()
}

func (r responseBodyStatus) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	body := r.session.responseBody
	graphemes := body.Content.Get()
	wrapLabel := "wrap off"
	if body.WrapMode.Get() != t.WrapNone {
		wrapLabel = "wrap on"
	}
	children := []t.Widget{}
	if r.session.responseVisual.Get() {
		children = append(children, t.Text{Content: " VISUAL ", Style: t.Style{ForegroundColor: theme.TextOnAccent, BackgroundColor: theme.Accent, Bold: true}})
	}
	lines := 1
	for _, g := range graphemes {
		if g == "\n" {
			lines++
		}
	}
	children = append(children,
		t.Text{Content: orDefault(r.contentType, "unknown type"), Style: t.Style{ForegroundColor: theme.AccentText}},
		t.Text{Content: pluralize(lines, "line"), Style: t.Style{ForegroundColor: theme.TextMuted}},
		t.Spacer{},
	)
	// The cursor is only drawn while the body has focus.
	if isFocusedID(ctx, "resp-body") {
		children = append(children, t.Text{Content: cursorPosition(graphemes, body.CursorIndex.Get()), Style: t.Style{ForegroundColor: theme.TextMuted}})
	}
	return t.Row{
		Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
		Spacing: 2,
		Children: append(children,
			t.Text{Content: wrapLabel, Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { body.ToggleWrap() }},
			t.Text{Content: "copy", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { r.copyBody() }},
		),
	}
}

// responseHeaders is a table of headers, or of trailers.
type responseHeaders struct {
	fillParent
	id     string
	state  *t.TableState[model.Header]
	scroll *t.ScrollState
	empty  string
}

func (h responseHeaders) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	rows := h.state.Rows.Get()
	if len(rows) == 0 {
		return emptyState{Title: h.empty}
	}
	focused := isFocusedID(ctx, h.id)
	return scrollingTable(h.scroll, t.Table[model.Header]{
		ID:            h.id,
		State:         h.state,
		SelectionMode: t.TableSelectionRow,
		Columns:       []t.TableColumn{{Width: t.Cells(nameColumnWidth(rows, func(h model.Header) string { return h.Name }))}, {Width: t.Flex(1)}},
		RenderCell: func(row model.Header, rowIndex, col int, active, selected bool) t.Widget {
			return tableCell(theme, active, focused, col == 0, []string{row.Name, row.Value}[col])
		},
		Style: t.Style{Width: t.Flex(1)},
	})
}

type responseCookies struct {
	fillParent
	session *Session
}

func (c responseCookies) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	if len(c.session.responseCookies.Rows.Get()) == 0 {
		return emptyState{Title: "No cookies", Lines: []string{"The response didn't set any cookies"}}
	}
	focused := isFocusedID(ctx, "resp-cookies")
	return scrollingTable(c.session.responseCookiesScroll, t.Table[model.Cookie]{
		ID:            "resp-cookies",
		State:         c.session.responseCookies,
		SelectionMode: t.TableSelectionRow,
		Columns: []t.TableColumn{
			{Width: t.Cells(nameColumnWidth(c.session.responseCookies.Rows.Get(), func(c model.Cookie) string { return c.Name })), Header: tableHeader(theme, "Name")},
			{Width: t.Flex(1), Header: tableHeader(theme, "Value")},
			{Width: t.Cells(14), Header: tableHeader(theme, "Path")},
			{Width: t.Cells(18), Header: tableHeader(theme, "Flags")},
		},
		RenderCell: func(row model.Cookie, rowIndex, col int, active, selected bool) t.Widget {
			var flags []string
			if row.HTTPOnly {
				flags = append(flags, "HttpOnly")
			}
			if row.Secure {
				flags = append(flags, "Secure")
			}
			return tableCell(theme, active, focused, col == 0, []string{row.Name, row.Value, row.Path, strings.Join(flags, " ")}[col])
		},
		Style: t.Style{Width: t.Flex(1)},
	})
}

// nameColumnWidth fits a table's name column to its longest name, within
// reason, plus the cell padding.
func nameColumnWidth[T any](rows []T, name func(T) string) int {
	width := 8
	for _, row := range rows {
		width = max(width, utf8.RuneCountInString(name(row)))
	}
	return min(width, 40) + 3
}

// Tables use no column spacing: cells pad themselves instead, so the cursor
// row is one unbroken bar rather than a block per column.

// tableCell draws one cell. The cursor row is only emphasised while the
// table has focus, as in the collection tree.
func tableCell(theme t.ThemeData, active, focused, key bool, content string) t.Widget {
	style := t.Style{Width: t.Flex(1), ForegroundColor: theme.Text, Padding: t.EdgeInsetsXY(1, 0)}
	if key {
		style.ForegroundColor = theme.PrimaryText
		style.Bold = true
	}
	switch {
	case active && focused:
		style.BackgroundColor = theme.ActiveCursor
		style.ForegroundColor = theme.SelectionText
	case active:
		style.BackgroundColor = theme.Surface
	}
	return t.Text{Content: content, Style: style}
}

// tableHeader is a column heading, padded to line up with tableCell.
func tableHeader(theme t.ThemeData, label string) t.Widget {
	return t.Text{Content: label, Style: t.Style{ForegroundColor: theme.TextMuted, Padding: t.EdgeInsetsXY(1, 0)}}
}

// responseTrace is a waterfall of the exchange's stages.
type responseTrace struct {
	fillParent
	session *Session
}

func (r responseTrace) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	events := r.session.trace.Get()
	if len(events) == 0 {
		return emptyState{Title: "No trace", Lines: []string{"Timing is recorded for requests sent in this session"}}
	}
	var total time.Duration
	for _, e := range events {
		total += e.Duration
	}
	const barWidth = 40
	rows := make([]t.Widget, 0, len(events)+1)
	var offset time.Duration
	for _, e := range events {
		start, width := 0, 0
		if total > 0 {
			start = int(float64(offset) / float64(total) * barWidth)
			width = max(1, int(float64(e.Duration)/float64(total)*barWidth))
		}
		color := theme.Success
		label := model.FormatDuration(e.Duration)
		switch e.State {
		case model.TraceFailed:
			color, label = theme.Error, "failed"
		case model.TraceStarted:
			color, label = theme.Warning, "waiting"
		case model.TraceSkipped:
			width, label = 0, "skipped"
		}
		// Durations come before the bar so they stay visible in narrow panels.
		rows = append(rows, t.Text{Spans: []t.Span{
			{Text: padRight(e.Stage.String(), 18), Style: t.SpanStyle{Foreground: theme.Text}},
			{Text: padLeft(label, 10) + "  ", Style: t.SpanStyle{Foreground: theme.TextMuted}},
			{Text: strings.Repeat(" ", start)},
			{Text: strings.Repeat("━", width), Style: t.SpanStyle{Foreground: color}},
		}})
		offset += e.Duration
	}
	rows = append(rows, t.Text{Spans: []t.Span{
		{Text: padRight("total", 18), Style: t.SpanStyle{Foreground: theme.Text, Bold: true}},
		{Text: padLeft(model.FormatDuration(total), 10), Style: t.SpanStyle{Foreground: theme.Text, Bold: true}},
	}})
	return t.Scrollable{
		ID:        "resp-trace-scroll",
		State:     r.session.traceScroll,
		Focusable: true,
		Style:     t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Child:     t.Column{Style: t.Style{Width: t.Flex(1), Padding: inset}, Children: rows},
	}
}

// formatBody pretty-prints JSON bodies and returns others unchanged.
func formatBody(resp *model.Response, prettify bool) string {
	if prettify && strings.Contains(resp.ContentType(), "json") {
		var out bytes.Buffer
		if err := json.Indent(&out, resp.Body, "", "  "); err == nil {
			return out.String()
		}
	}
	return string(resp.Body)
}

func escapeMarkup(s string) string {
	return strings.ReplaceAll(s, "[", "[[")
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
