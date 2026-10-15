package ui

import (
	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
)

const webSocketMessageID = "req-ws-message"

type webSocketEditor struct {
	session   *Session
	message   *t.TextAreaState
	scroll    *t.ScrollState
	binary    *t.CheckboxState
	variables *completion
}

func newWebSocketEditor(s *Session) payloadEditor {
	return &webSocketEditor{session: s, message: t.NewTextAreaState(""), scroll: t.NewScrollState(), binary: t.NewCheckboxState(false), variables: newCompletion()}
}

func (e *webSocketEditor) load(req model.Request) {
	w, _ := req.Payload.(model.WebSocket)
	e.message.SetText(w.Message)
	e.message.ClearSelection()
	e.message.CursorIndex.Set(0)
	e.scroll.SetOffset(0)
	e.binary.Checked.Set(w.Binary)
}

func (e *webSocketEditor) payload() model.Payload {
	return model.WebSocket{Message: e.message.GetText(), Binary: e.binary.Checked.Get()}
}

func (e *webSocketEditor) tabs() []requestTab {
	return []requestTab{{key: "ws-message", label: "Message", jump: "i", marked: func() bool { return hasText(e.message) }}}
}

func (e *webSocketEditor) focusID(string) string { return webSocketMessageID }
func (e *webSocketEditor) view(_ string, a *App) t.Widget {
	return webSocketMessageView{app: a, editor: e}
}

type webSocketMessageView struct {
	fillParent
	app    *App
	editor *webSocketEditor
}

func (v webSocketMessageView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, s, a := v.editor, v.editor.session, v.app
	substitute := s.substitute.Checked.Get()
	language, placeholder := "json", "Write a text or JSON message…"
	if e.binary.Checked.Get() {
		language, placeholder = "text", "Base64-encoded binary message…"
	}
	var area t.Widget = t.TextArea{
		ID: webSocketMessageID, State: e.message, ScrollState: e.scroll, Placeholder: placeholder,
		Highlighter:   a.bodyHighlighter(theme, language, substitute),
		Style:         t.Style{Width: t.Flex(1), BackgroundColor: theme.Background, Padding: inset},
		OnChange:      func(string) { s.touch() },
		ExtraKeybinds: a.externalKeybinds(e.message, language, s.touch),
	}
	if substitute {
		area = e.variables.wrap(theme, area, a.variableChoices(), t.Flex(1))
	}
	label := "Connect first; the draft is not sent on connection"
	if s.streaming.Get() == streamOpen {
		label = a.keyHint("send-request") + " Send message · " + a.keyHint("stream") + " Disconnect"
	}
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Spacing: 1, Children: []t.Widget{
		&t.Checkbox{ID: "req-ws-binary", State: e.binary, Label: "Binary (base64)", OnChange: func(bool) { s.touch() }, Style: t.Style{Padding: inset}},
		scrollingArea(webSocketMessageID, e.scroll, theme.Background, area),
		t.Text{Content: label, Style: t.Style{ForegroundColor: theme.TextMuted, Padding: inset}},
	}}
}
