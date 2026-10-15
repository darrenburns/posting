package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

const messagesID = "resp-messages"

// messageLog keeps navigation separate from arriving data. Rows are append
// only during a call, so inspecting an older message never changes its body.
type messageLog struct {
	list      *t.ListState[messageRow]
	scroll    *t.ScrollState
	split     *t.SplitPaneState
	narrow    t.Signal[bool]
	detail    t.Signal[bool]
	following t.Signal[bool]
	unread    t.Signal[int]
	shown     int
}

type messageRow struct {
	event   model.MessageEvent
	index   int
	preview string
}

func newMessageLog() *messageLog {
	return &messageLog{
		list: t.NewListState[messageRow](nil), scroll: t.NewScrollState(),
		split: t.NewSplitPaneState(0.5), narrow: t.NewSignal(false), detail: t.NewSignal(false),
		following: t.NewSignal(true), unread: t.NewSignal(0), shown: -1,
	}
}

func (m *messageLog) update(s *Session, events []model.MessageEvent, reset bool) {
	if reset {
		m.list.Clear()
		m.scroll.SetOffset(0)
		m.detail.Set(false)
		m.following.Set(true)
		m.unread.Set(0)
		m.shown = -1
	}
	rows := append([]messageRow(nil), m.list.GetItems()...)
	before := len(rows)
	for i := before; i < len(events); i++ {
		event := events[i]
		preview := event.Title
		if preview == "" {
			preview = string(event.Body[:min(len(event.Body), 256)])
		}
		if event.Event != "" {
			preview = event.Event + " · " + preview
		}
		rows = append(rows, messageRow{event: event, index: i, preview: strings.Join(strings.Fields(preview), " ")})
	}
	m.scroll.PinToBottom = m.following.Peek()
	m.list.SetItems(rows)
	if m.following.Peek() {
		m.followLatest(s)
	} else {
		m.unread.Set(m.unread.Peek() + len(rows) - before)
	}
}

func (m *messageLog) followLatest(s *Session) {
	m.following.Set(true)
	m.unread.Set(0)
	m.list.SelectLast()
	// Keep following when layout measures the newly appended rows.
	m.scroll.PinToBottom = true
	m.scroll.ScrollToBottom()
	m.showSelected(s)
}

func (m *messageLog) showSelected(s *Session) {
	row, ok := m.list.SelectedItem()
	if !ok || row.index == m.shown {
		return
	}
	m.shown = row.index
	resp := messageResponse(row.event)
	s.responseBody.SetText(formatBody(resp, s.prettifyJSON))
	s.setResponseJSON(resp.Body)
	s.responseBody.CursorIndex.Set(0)
	s.responseBody.ClearSelection()
	s.responseBodyScroll.SetOffset(0)
	s.responseVisual.Set(false)
}

func messageResponse(event model.MessageEvent) *model.Response {
	body, contentType := event.Body, event.ContentType
	if len(body) == 0 && event.Event == "" {
		body, contentType = []byte(event.Title), "text/plain"
	}
	return &model.Response{Body: body, BodyContentType: contentType}
}

func messageLabel(kind model.MessageEventKind) string {
	switch kind {
	case model.MessageSent:
		return "↑"
	case model.MessageReceived:
		return "↓"
	case model.MessageError:
		return "! Error"
	default:
		return "· Event"
	}
}

func messageColor(theme t.ThemeData, kind model.MessageEventKind) t.Color {
	switch kind {
	case model.MessageSent:
		return theme.WarningText
	case model.MessageReceived:
		return theme.SuccessText
	case model.MessageError:
		return theme.ErrorText
	default:
		return theme.TextMuted
	}
}

type responseMessages struct {
	fillParent
	app     *App
	session *Session
}

func (v responseMessages) Keybinds() []t.Keybind {
	return []t.Keybind{{Key: "f", Name: "Follow latest", Action: func() {
		v.session.messages.followLatest(v.session)
	}}}
}

func (v responseMessages) Build(ctx t.BuildContext) t.Widget {
	s, theme := v.session, ctx.Theme()
	m := s.messages
	m.scroll.PinToBottom = m.following.Get()
	inspect := func(messageRow) {
		m.following.Set(false)
		m.detail.Set(true)
		t.RequestFocus("resp-body")
	}
	list := t.Scrollable{
		State: m.scroll, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		ScrollbarFadeDelay:  scrollbarFadeDelay,
		ScrollbarTrackColor: t.RGBA(0, 0, 0, 0),
		MouseWheel:          func(t.MouseEvent) bool { m.following.Set(false); return false },
		Child: t.List[messageRow]{
			ID: messagesID, State: m.list, ScrollState: m.scroll, ItemHeight: 1,
			OnSelect: inspect, ActivateOnClick: m.narrow.Get(),
			OnCursorChange: func(messageRow) { m.following.Set(false); m.showSelected(s) },
			RenderItem: func(row messageRow, active, _ bool) t.Widget {
				bg, fg := theme.Background, theme.TextMuted
				if active {
					bg, fg = theme.Surface, theme.Text
				}
				return t.Row{
					Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: bg, Padding: inset},
					Spacing: 1,
					Children: []t.Widget{
						t.Text{Content: messageLabel(row.event.Kind), Style: t.Style{ForegroundColor: messageColor(theme, row.event.Kind)}},
						messagePreview{t.Text{Content: row.preview, Style: t.Style{Width: t.Flex(1), ForegroundColor: fg}}},
						t.Text{Content: row.event.At.Local().Format("15:04:05.000"), Style: t.Style{Width: t.Cells(12), ForegroundColor: theme.TextMuted.WithAlpha(0.64)}},
					},
				}
			},
			Style: t.Style{Width: t.Flex(1)},
		},
	}
	var content t.Widget = list
	inspector := messageInspector{app: v.app, session: s}
	switch {
	case m.detail.Get():
		content = inspector
	case !m.narrow.Get():
		content = t.SplitPane{
			ID: "resp-message-split", State: m.split, First: list, Second: inspector,
			Orientation: t.SplitHorizontal, MinPaneSize: 24, DisableFocus: true,
		}
	}
	follow := "f Follow latest"
	followColor := theme.AccentText
	if m.following.Get() {
		follow = "Following latest"
		followColor = theme.TextMuted
	} else if n := m.unread.Get(); n > 0 {
		follow = fmt.Sprintf("%d new · f Follow latest", n)
	}
	hint := "↑↓ Select · enter Inspect"
	if m.detail.Get() {
		hint = "esc Back to log"
	} else if m.narrow.Get() && s.phase.Get() == exchangeSending && s.response.Get().ContentType() == "text/event-stream" {
		hint = "enter Inspect · esc Stop"
	}
	return messageLogLayout{
		state: m,
		Column: t.Column{
			Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Children: []t.Widget{
				content,
				t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: inset}, Children: []t.Widget{
					t.Text{Content: hint, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}},
					t.Text{Content: follow, Style: t.Style{ForegroundColor: followColor}, Click: func(t.MouseEvent) { m.followLatest(s) }},
				}},
			},
		},
	}
}

// Elide at the laid-out width, including after dragging the split divider.
type messagePreview struct{ t.Text }

func (p messagePreview) Build(t.BuildContext) t.Widget { return p }
func (p messagePreview) Render(ctx *t.RenderContext) {
	p.Content = ansi.Truncate(p.Content, ctx.Width, "…")
	p.Text.Render(ctx)
}

// The same inspector works with any decoded message. It uses the body
// viewer's selection, copy, highlighting and external-editor bindings.
type messageInspector struct {
	fillParent
	app     *App
	session *Session
}

func (v messageInspector) Keybinds() []t.Keybind {
	return []t.Keybind{{Key: "escape", Name: "Back to log", Action: func() {
		v.session.messages.detail.Set(false)
		t.RequestFocus(messagesID)
	}}}
}

func (v messageInspector) Build(ctx t.BuildContext) t.Widget {
	m, theme := v.session.messages, ctx.Theme()
	m.list.CursorIndex.Get()
	row, ok := m.list.SelectedItem()
	if !ok {
		return emptyState{Title: "No messages yet"}
	}
	label := messageLabel(row.event.Kind)
	switch row.event.Kind {
	case model.MessageSent:
		label += " Sent"
	case model.MessageReceived:
		label += " Received"
	}
	var body t.Widget = responseBody{app: v.app, session: v.session, response: messageResponse(row.event)}
	if row.event.Event != "" {
		metadata := row.event.Event
		if row.event.ID != "" {
			metadata += " · ID: " + row.event.ID
		}
		if len(row.event.Body) == 0 {
			body = t.Scrollable{
				ID: "resp-body", State: v.session.responseBodyScroll, Focusable: true,
				Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
				Child: emptyState{Title: "Empty event data"},
			}
		}
		body = t.Column{
			Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Children: []t.Widget{
				messagePreview{t.Text{Content: metadata, Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), ForegroundColor: theme.TextMuted}}},
				body,
			},
		}
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{
			t.Row{
				Style:   t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: inset},
				Spacing: 1,
				Children: []t.Widget{
					t.Text{
						Spans: []t.Span{
							{Text: label, Style: t.SpanStyle{Foreground: messageColor(theme, row.event.Kind)}},
							{Text: fmt.Sprintf(" · #%d", row.index+1)},
						},
						Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted},
					},
					t.Text{Content: row.event.At.Local().Format("15:04:05.000"), Style: t.Style{Width: t.Cells(12), ForegroundColor: theme.TextMuted.WithAlpha(0.8)}},
				},
			},
			body,
		},
	}
}

// Inspect in place on wide panes; on narrow panes enter opens the full
// payload. Measure the actual response pane, not the terminal's width.
type messageLogLayout struct {
	t.Column
	state *messageLog
}

func (v messageLogLayout) Build(t.BuildContext) t.Widget { return v }
func (v messageLogLayout) ChildWidgets() []t.Widget      { return v.Children }
func (v messageLogLayout) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	if narrow := metrics.Box().ContentWidth() < 78; narrow != v.state.narrow.Peek() {
		t.Dispatch(func() { v.state.narrow.Set(narrow) })
	}
}
