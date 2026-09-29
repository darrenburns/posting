package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	title, subtitle := "", ""
	if resp != nil {
		fg, bg := statusColors(theme, resp.StatusCode)
		title = fmt.Sprintf(" [b %s on %s] %d %s [/]", fg.Hex(), bg.Hex(), resp.StatusCode, resp.Reason)
		if p.app.settings.Response.ShowSizeAndTime {
			subtitle = fmt.Sprintf("[$Text]%s[/] [$TextMuted]in[/] [$Text]%s[/]", model.FormatBytes(resp.Size()), model.FormatDuration(resp.Elapsed))
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
	case resp == nil && phase == exchangeSending:
		content = t.Column{
			Style:      t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			MainAlign:  t.MainAxisCenter,
			CrossAlign: t.CrossAxisCenter,
			Children: []t.Widget{
				t.Row{Spacing: 1, Children: []t.Widget{
					t.Spinner{State: s.spinner, Style: t.Style{ForegroundColor: theme.AccentText}},
					t.Text{Content: "Sending request…", Style: t.Style{ForegroundColor: theme.Text}},
				}},
				t.ParseMarkupToText("[$TextMuted]Press [b]esc[/] to cancel[/]", theme),
			},
		}
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

type responseTabs struct {
	fillParent
	app      *App
	session  *Session
	response *model.Response
}

// responseTabs is the response panel's tab strip for resp.
func (s *Session) responseTabs(resp *model.Response) tabStrip {
	return tabStrip{
		ID:     responseTabsID,
		Active: s.responseTab,
		View:   s.responseTabView,
		Tabs: []tabItem{
			{Key: "body", Label: "Body"},
			{Key: "headers", Label: "Headers", Badge: countBadge(len(resp.Headers))},
			{Key: "cookies", Label: "Cookies", Badge: countBadge(len(resp.Cookies))},
			{Key: "trace", Label: "Trace"},
		},
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
				"body":    responseBody{app: r.app, session: s, response: resp},
				"headers": responseHeaders{session: s},
				"cookies": responseCookies{session: s},
				"trace":   responseTrace{session: s},
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
		return emptyState{Title: "Empty body", Lines: []string{fmt.Sprintf("The server returned %d %s with no content", resp.StatusCode, resp.Reason)}}
	}
	contentType := resp.ContentType()
	wrap := s.responseBody.WrapMode.Get() != t.WrapNone
	wrapLabel := "wrap off"
	if wrap {
		wrapLabel = "wrap on"
	}
	lines := strings.Count(s.responseBody.GetText(), "\n") + 1
	toggleWrap := func() { s.responseBody.ToggleWrap() }
	copyBody := func() {
		t.SetClipboard('c', s.responseBody.GetText())
		b.app.notify("Copied response body", toastSuccess)
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{
			t.TextArea{
				ID:          "resp-body",
				State:       s.responseBody,
				ScrollState: s.responseBodyScroll,
				Highlighter: b.app.bodyHighlighter(theme, languageFor(contentType), false),
				Style:       t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background, Padding: t.EdgeInsetsXY(1, 0)},
				ExtraKeybinds: []t.Keybind{
					{Key: "w", Name: "Toggle wrap", Action: toggleWrap},
					{Key: "y", Name: "Copy body", Action: copyBody},
				},
			},
			t.Row{
				Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
				Spacing: 2,
				Children: []t.Widget{
					t.Text{Content: orDefault(contentType, "unknown type"), Style: t.Style{ForegroundColor: theme.AccentText}},
					t.Text{Content: pluralize(lines, "line"), Style: t.Style{ForegroundColor: theme.TextMuted}},
					t.Spacer{},
					t.Text{Content: wrapLabel, Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { toggleWrap() }},
					t.Text{Content: "copy", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { copyBody() }},
				},
			},
		},
	}
}

type responseHeaders struct {
	fillParent
	session *Session
}

func (h responseHeaders) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	if len(h.session.responseHeaders.Rows.Get()) == 0 {
		return emptyState{Title: "No headers"}
	}
	focused := isFocusedID(ctx, "resp-headers")
	return t.Table[model.Header]{
		ID:            "resp-headers",
		State:         h.session.responseHeaders,
		SelectionMode: t.TableSelectionRow,
		Columns:       []t.TableColumn{{Width: t.Cells(30)}, {Width: t.Flex(1)}},
		RenderCell: func(row model.Header, rowIndex, col int, active, selected bool) t.Widget {
			return tableCell(theme, active, focused, col == 0, []string{row.Name, row.Value}[col])
		},
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
	}
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
	return t.Table[model.Cookie]{
		ID:            "resp-cookies",
		State:         c.session.responseCookies,
		SelectionMode: t.TableSelectionRow,
		Columns: []t.TableColumn{
			{Width: t.Cells(22), Header: tableHeader(theme, "Name")},
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
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
	}
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
