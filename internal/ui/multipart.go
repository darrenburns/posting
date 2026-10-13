package ui

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
)

const partsID = "req-multipart-parts"

type partEditor struct {
	kind                                       t.Signal[model.PartType]
	enabled                                    t.Signal[bool]
	name, filename, contentType                *t.TextInputState
	text                                       *t.TextAreaState
	textScroll                                 *t.ScrollState
	path                                       *uploadPath
	nameVars, textVars, filenameVars, typeVars *completion
	scroll                                     *formScroll
}

func newPartEditor(p model.Part) *partEditor {
	if p.Type == "" {
		p.Type = model.PartText
	}
	return &partEditor{kind: t.NewSignal(p.Type), enabled: t.NewSignal(p.Enabled), name: t.NewTextInputState(p.Name), filename: t.NewTextInputState(p.Filename), contentType: t.NewTextInputState(p.ContentType),
		text: t.NewTextAreaState(p.Value), textScroll: t.NewScrollState(), path: newUploadPath(p.File), scroll: newFormScroll(),
		nameVars: newCompletion(), textVars: newCompletion(), filenameVars: newCompletion(), typeVars: newCompletion()}
}

func (p *partEditor) value() model.Part {
	v := model.Part{Type: p.kind.Peek(), Name: p.name.GetText(), Enabled: p.enabled.Peek(), Filename: p.filename.GetText(), ContentType: p.contentType.GetText()}
	if v.Type == model.PartFile {
		v.File = p.path.text.GetText()
	} else {
		v.Value = p.text.GetText()
	}
	return v
}

type multipartEditor struct {
	list           *t.ListState[*partEditor]
	scroll         *t.ScrollState
	split          *t.SplitPaneState
	narrow, detail t.Signal[bool]
}

func newMultipartEditor() *multipartEditor {
	return &multipartEditor{list: t.NewListState[*partEditor](nil), scroll: t.NewScrollState(), split: t.NewSplitPaneState(0.34), narrow: t.NewSignal(false), detail: t.NewSignal(false)}
}
func (e *multipartEditor) load(parts []model.Part) {
	var rows []*partEditor
	for _, p := range parts {
		rows = append(rows, newPartEditor(p))
	}
	e.list.SetItems(rows)
	e.list.SelectFirst()
	e.detail.Set(false)
}
func (e *multipartEditor) values() []model.Part {
	var out []model.Part
	for _, p := range e.list.GetItems() {
		out = append(out, p.value())
	}
	return out
}

type multipartView struct {
	fillParent
	app     *App
	session *Session
}

func (v multipartView) add() {
	e := v.session.multipart
	e.list.Append(newPartEditor(model.Part{Type: model.PartText, Enabled: true}))
	e.list.SelectLast()
	e.detail.Set(true)
	v.session.touch()
	t.RequestFocus("req-part-name")
}
func (v multipartView) remove() {
	e := v.session.multipart
	e.list.RemoveAt(e.list.CursorIndex.Peek())
	v.session.touch()
	e.detail.Set(false)
	t.RequestFocus(partsID)
}
func (v multipartView) toggle() {
	if p, ok := v.session.multipart.list.SelectedItem(); ok {
		p.enabled.Set(!p.enabled.Peek())
		v.session.touch()
	}
}
func (v multipartView) move(delta int) {
	e := v.session.multipart
	i := e.list.CursorIndex.Peek()
	rows := slices.Clone(e.list.GetItems())
	j := i + delta
	if i < 0 || i >= len(rows) || j < 0 || j >= len(rows) {
		return
	}
	rows[i], rows[j] = rows[j], rows[i]
	e.list.SetItems(rows)
	e.list.SelectIndex(j)
	v.session.touch()
}
func (v multipartView) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "insert", Name: "Add part", Action: v.add},
		{Key: "ctrl+x", Name: "Delete part", Action: v.remove},
		{Key: "ctrl+r", Name: "Toggle part", Action: v.toggle},
		{Key: "alt+up", Name: "Move part up", Action: func() { v.move(-1) }},
		{Key: "alt+down", Name: "Move part down", Action: func() { v.move(1) }},
	}
}
func (v multipartView) Build(ctx t.BuildContext) t.Widget {
	s := v.session
	e := s.multipart
	e.list.CursorIndex.Get()
	inspect := func(*partEditor) { e.detail.Set(true); t.RequestFocus("req-part-name") }
	list := t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: []t.Widget{
		t.Scrollable{State: e.scroll, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Child: t.List[*partEditor]{ID: partsID, State: e.list, ScrollState: e.scroll, ItemHeight: 2, OnSelect: inspect, ActivateOnClick: e.narrow.Get(),
			RenderItem: func(p *partEditor, active, _ bool) t.Widget {
				return multipartRow{part: p, active: active, app: v.app, session: s}
			}, Style: t.Style{Width: t.Flex(1)}}},
		t.Row{Spacing: 1, Children: []t.Widget{
			t.Button{ID: "req-part-add", Label: "+ Add part", OnPress: v.add},
			t.Button{ID: "req-part-delete", Label: "Delete", OnPress: v.remove},
		}},
	}}
	var content t.Widget = list
	inspector := partInspector{view: v}
	if e.narrow.Get() {
		if e.detail.Get() {
			content = inspector
		}
	} else {
		content = t.SplitPane{ID: "req-parts-split", State: e.split, First: list, Second: inspector, Orientation: t.SplitHorizontal, MinPaneSize: 23, DisableFocus: true}
	}
	return multipartLayout{state: e, Column: t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: []t.Widget{content}}}
}

type multipartRow struct {
	fillWidth
	part    *partEditor
	active  bool
	app     *App
	session *Session
}

func (v multipartRow) Build(ctx t.BuildContext) t.Widget {
	p, theme := v.part, ctx.Theme()
	bg, fg := theme.Background, theme.Text
	if v.active {
		bg = theme.Surface
	}
	mark := "☑"
	if !p.enabled.Get() {
		mark = "☐"
		fg = theme.TextMuted
	}
	name := p.name.GetText()
	if name == "" {
		name = "Unnamed part"
	}
	kind, summary := "Text", strings.Join(strings.Fields(p.text.GetText()), " ")
	if p.kind.Get() == model.PartFile {
		kind, summary = "File", filepath.Base(p.path.text.GetText())
		if p.enabled.Get() {
			p.path.inspect(v.app, v.session)
			if p.path.problem.Get() {
				mark = "!"
				fg = theme.ErrorText
			}
		}
	}
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Cells(2), BackgroundColor: bg, Padding: inset}, Children: []t.Widget{
		t.Row{Style: t.Style{Width: t.Flex(1)}, Spacing: 1, Children: []t.Widget{
			messagePreview{t.Text{Content: mark + " " + name, Style: t.Style{Width: t.Flex(1), ForegroundColor: fg}}}, t.Text{Content: kind, Style: t.Style{ForegroundColor: theme.TextMuted}},
		}},
		messagePreview{t.Text{Content: "  " + summary, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}}},
	}}
}

type partInspector struct {
	fillParent
	view multipartView
}

func (v partInspector) back() { v.view.session.multipart.detail.Set(false); t.RequestFocus(partsID) }
func (v partInspector) Keybinds() []t.Keybind {
	if v.view.session.phase.Get() == exchangeSending {
		return nil // Let the global Escape binding cancel the request.
	}
	return []t.Keybind{{Key: "escape", Name: "Back to parts", Action: v.back}}
}
func (v partInspector) Build(ctx t.BuildContext) t.Widget {
	s, a, theme := v.view.session, v.view.app, ctx.Theme()
	e := s.multipart
	e.list.CursorIndex.Get()
	p, ok := e.list.SelectedItem()
	if !ok {
		return emptyState{Title: "No parts yet", Lines: []string{"Choose [b]+ Add part[/] to add text or a file"}}
	}
	choices := variableChoices{}
	if s.substitute.Checked.Get() {
		choices = a.variableChoices()
	}
	highlighter := variableHighlighter(theme, a.resolver())
	edit := func(string) { s.touch() }
	rows := []formField{}
	if e.narrow.Get() {
		rows = append(rows, field(t.Button{ID: "req-part-back", Label: "‹ Parts", OnPress: v.back, Click: func(t.MouseEvent) { v.back() }}, "req-part-back"))
	}
	rows = append(rows,
		field(formRow(ctx, "Name", "", input{ID: "req-part-name", State: p.name, Placeholder: "Field name", OnChange: edit, Completion: p.nameVars, Choices: choices, Highlighter: highlighter}), "req-part-name"),
		field(formRow(ctx, "Type", "", t.Row{Spacing: 1, Children: []t.Widget{
			tabStrip{ID: "req-part-type", Tabs: []tabItem{{Key: "text", Label: "Text"}, {Key: "file", Label: "File"}}, Active: string(p.kind.Get()), OnChange: func(value string) { p.kind.Set(model.PartType(value)); s.touch() }},
			t.Button{ID: "req-part-toggle", Label: map[bool]string{true: "☑ Enabled", false: "☐ Disabled"}[p.enabled.Get()], OnPress: v.view.toggle, Click: func(t.MouseEvent) { v.view.toggle() }},
		}}), "req-part-type", "req-part-toggle"),
	)
	if p.kind.Get() == model.PartFile {
		p.path.inspect(a, s)
		rows = append(rows,
			field(formRow(ctx, "File path", "", uploadPathField{path: p.path, app: a, session: s, id: "req-part-path", changed: s.touch}), "req-part-path"),
		)
	}
	// curl can attach a filename to a text part too. Never hide an override
	// that is still sent, including after switching a file part to text.
	if p.kind.Get() == model.PartFile || p.filename.GetText() != "" {
		rows = append(rows, field(formRow(ctx, "Filename", "", input{ID: "req-part-filename", State: p.filename, Placeholder: "Auto: " + filepath.Base(p.path.text.GetText()), OnChange: edit, Completion: p.filenameVars, Choices: choices, Highlighter: highlighter}), "req-part-filename"))
	}
	auto := "text/plain"
	if p.kind.Get() == model.PartFile {
		auto = model.FileContentType(a.resolvedUploadPath(s, p.path.text.GetText()))
	}
	rows = append(rows, field(formRow(ctx, "Content type", "", input{ID: "req-part-content-type", State: p.contentType, Placeholder: "Auto: " + auto, OnChange: edit, Completion: p.typeVars, Choices: choices, Highlighter: highlighter}), "req-part-content-type"))
	if p.kind.Get() == model.PartFile {
		rows = append(rows, field(t.Text{Content: "Paths are relative to the collection.", Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted, Padding: inset}}))
	} else {
		language := languageFor(p.contentType.GetText())
		var area t.Widget = t.TextArea{ID: "req-part-text", State: p.text, ScrollState: p.textScroll, Placeholder: "Part value…", Highlighter: a.bodyHighlighter(theme, language, s.substitute.Checked.Get()), Style: t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: inset}, OnChange: edit, ExtraKeybinds: a.externalKeybinds(p.text, language, s.touch)}
		if s.substitute.Checked.Get() {
			area = p.textVars.wrap(theme, area, choices, t.Flex(1))
		}
		rows = append(rows, field(t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Cells(7)}, Children: []t.Widget{
			t.Text{Content: "Value", Style: t.Style{ForegroundColor: theme.Text}}, scrollingArea("req-part-text", p.textScroll, theme.Surface, area),
		}}, "req-part-text"))
	}
	return scrollForm{State: p.scroll, Rows: rows, Spacing: 1}
}

type multipartLayout struct {
	t.Column
	state *multipartEditor
}

func (v multipartLayout) Build(t.BuildContext) t.Widget { return v }
func (v multipartLayout) ChildWidgets() []t.Widget      { return v.Children }
func (v multipartLayout) OnLayout(_ t.BuildContext, m t.LayoutMetrics) {
	if narrow := m.Box().ContentWidth() < 78; narrow != v.state.narrow.Peek() {
		t.Dispatch(func() { v.state.narrow.Set(narrow) })
	}
}

type fileBodyEditor struct {
	fillParent
	app     *App
	session *Session
}

func (v fileBodyEditor) Build(ctx t.BuildContext) t.Widget {
	s, a, theme := v.session, v.app, ctx.Theme()
	p := s.upload
	p.inspect(a, s)
	choices := variableChoices{}
	if s.substitute.Checked.Get() {
		choices = a.variableChoices()
	}
	return scrollForm{State: s.uploadScroll, Spacing: 1, Rows: []formField{
		field(formRow(ctx, "File path", "", uploadPathField{path: p, app: a, session: s, id: "req-file-path", changed: s.touch}), "req-file-path"),
		field(formRow(ctx, "Content type", "", input{ID: "req-file-content-type", State: s.uploadType, Placeholder: "Auto: " + model.FileContentType(a.resolvedUploadPath(s, p.text.GetText())), OnChange: func(string) { s.touch() }, Completion: s.uploadTypeVars, Choices: choices}), "req-file-content-type"),
		field(t.Text{Content: "The file is the entire request body, without multipart framing. Paths are relative to the collection; each send reads the current file.", Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted, Padding: inset}}),
	}}
}
