package ui

import (
	"fmt"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// kvRow is one editable name/value row.
type kvRow struct {
	id      int
	enabled t.Signal[bool]
	key     *t.TextInputState
	value   *t.TextInputState
	// keySuggestions completes the name column, when the editor has suggestions.
	keySuggestions *t.AutocompleteState
	// values completes ${VARIABLES} in the value column.
	values *completion
}

func (r *kvRow) empty() bool {
	return r.key.GetText() == "" && r.value.GetText() == ""
}

// kvEditor holds the rows for headers, query parameters, path parameters and
// form fields. Rows are edited in place like a spreadsheet; the last row is
// always blank and typing into it creates a new row.
type kvEditor struct {
	prefix    string
	rows      t.AnySignal[[]*kvRow]
	scroll    *t.ScrollState
	nextID    int
	fixedKeys bool // path parameters: names come from the URL
	onChange  func()
	// suggestions are offered while typing a name.
	suggestions []t.Suggestion
}

func newKVEditor(prefix string, fixedKeys bool, suggestions []t.Suggestion) *kvEditor {
	e := &kvEditor{
		prefix:    prefix,
		rows:      t.NewAnySignal[[]*kvRow](nil),
		scroll:    t.NewScrollState(),
		fixedKeys: fixedKeys, suggestions: suggestions,
	}
	e.ensureTrailing()
	return e
}

func (e *kvEditor) newRow(item model.KeyValue) *kvRow {
	e.nextID++
	row := &kvRow{
		id:      e.nextID,
		enabled: t.NewSignal(item.Enabled),
		key:     t.NewTextInputState(item.Name),
		value:   t.NewTextInputState(item.Value),
		values:  newCompletion(),
	}
	if len(e.suggestions) > 0 {
		row.keySuggestions = t.NewAutocompleteState()
		row.keySuggestions.SetSuggestions(e.suggestions)
	}
	return row
}

// Load replaces every row.
func (e *kvEditor) Load(items []model.KeyValue) {
	rows := make([]*kvRow, 0, len(items)+1)
	for _, item := range items {
		rows = append(rows, e.newRow(item))
	}
	e.rows.Set(rows)
	e.ensureTrailing()
}

// Values returns the non-blank rows.
func (e *kvEditor) Values() []model.KeyValue {
	var out []model.KeyValue
	for _, row := range e.rows.Peek() {
		if row.key.GetText() == "" {
			continue
		}
		out = append(out, model.KeyValue{Name: row.key.GetText(), Value: row.value.GetText(), Enabled: row.enabled.Peek()})
	}
	return out
}

// Count is the number of non-blank rows, read reactively.
func (e *kvEditor) Count() int {
	n := 0
	for _, row := range e.rows.Get() {
		if !isBlank(row.key) {
			n++
		}
	}
	return n
}

// ensureTrailing keeps exactly one blank row available at the end.
func (e *kvEditor) ensureTrailing() {
	if e.fixedKeys {
		return
	}
	rows := e.rows.Peek()
	if len(rows) == 0 || !rows[len(rows)-1].empty() {
		e.rows.Set(append(rows, e.newRow(model.KeyValue{Enabled: true})))
	}
}

func (e *kvEditor) changed() {
	e.ensureTrailing()
	if e.onChange != nil {
		e.onChange()
	}
}

func (e *kvEditor) remove(id int) {
	rows := e.rows.Peek()
	for i, row := range rows {
		if row.id != id {
			continue
		}
		next := append(append([]*kvRow(nil), rows[:i]...), rows[i+1:]...)
		e.rows.Set(next)
		e.changed()
		if focus := e.rowAt(i); focus != nil {
			t.RequestFocus(e.inputID(focus.id, "key"))
		}
		return
	}
}

func (e *kvEditor) toggle(id int) {
	for _, row := range e.rows.Peek() {
		if row.id == id && !row.empty() {
			row.enabled.Set(!row.enabled.Peek())
			e.changed()
		}
	}
}

func (e *kvEditor) rowAt(i int) *kvRow {
	rows := e.rows.Peek()
	if i < 0 || i >= len(rows) {
		return nil
	}
	return rows[i]
}

func (e *kvEditor) indexOf(id int) int {
	for i, row := range e.rows.Peek() {
		if row.id == id {
			return i
		}
	}
	return -1
}

func (e *kvEditor) inputID(rowID int, column string) string {
	return fmt.Sprintf("%s-%d-%s", e.prefix, rowID, column)
}

// FirstInputID is where focus lands when jumping into the editor.
func (e *kvEditor) FirstInputID() string {
	row := e.rowAt(0)
	if row == nil {
		return ""
	}
	if e.fixedKeys {
		return e.inputID(row.id, "value")
	}
	return e.inputID(row.id, "key")
}

// moveFocus moves to the same column in the row delta rows away.
func (e *kvEditor) moveFocus(rowID int, column string, delta int) {
	i := e.indexOf(rowID) + delta
	target := e.rowAt(i)
	if target == nil {
		return
	}
	if e.fixedKeys {
		column = "value"
	}
	e.scroll.ScrollToView(i, 1)
	t.RequestFocus(e.inputID(target.id, column))
}

// kvEditorView renders a kvEditor.
type kvEditorView struct {
	fillParent
	Editor           *kvEditor
	KeyPlaceholder   string
	ValuePlaceholder string
	Empty            emptyState
	KeyHighlighter   t.Highlighter
	ValueHighlighter t.Highlighter
	// Choices are offered as ${VARIABLE} completions in the value column.
	Choices variableChoices
}

func (v kvEditorView) Build(ctx t.BuildContext) t.Widget {
	e := v.Editor
	rows := e.rows.Get()
	if len(rows) == 0 {
		return v.Empty
	}
	children := make([]t.Widget, 0, len(rows))
	for _, row := range rows {
		children = append(children, kvRowView{editor: e, row: row, view: v})
	}
	return t.Scrollable{
		State: e.scroll,
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Child: t.Column{Style: t.Style{Width: t.Flex(1)}, Children: children},
	}
}

type kvRowView struct {
	fillWidth
	editor *kvEditor
	row    *kvRow
	view   kvEditorView
}

func (r kvRowView) Keybinds() []t.Keybind {
	if r.editor.fixedKeys {
		return nil
	}
	return []t.Keybind{
		{Key: "ctrl+x", Name: "Delete row", Action: func() { r.editor.remove(r.row.id) }},
		{Key: "ctrl+space", Name: "Toggle row", Action: func() { r.editor.toggle(r.row.id) }},
	}
}

func (r kvRowView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, row := r.editor, r.row
	enabled := row.enabled.Get()
	blank := isBlank(row.key) && isBlank(row.value)

	nav := func(column string) []t.Keybind {
		return []t.Keybind{
			{Key: "up", Name: "Row above", Action: func() { e.moveFocus(row.id, column, -1) }, Hidden: true},
			{Key: "down", Name: "Row below", Action: func() { e.moveFocus(row.id, column, 1) }, Hidden: true},
		}
	}
	onEdit := func(string) { e.changed() }

	var toggle, key, remove t.Widget
	if e.fixedKeys {
		toggle = t.Text{Content: " : ", Style: t.Style{ForegroundColor: theme.TextMuted}}
		key = t.Text{Content: row.key.GetText(), Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.Text, Bold: true, Padding: t.EdgeInsetsXY(1, 0)}}
		remove = t.EmptyWidget{}
	} else {
		mark, markColor := " ☐ ", theme.TextMuted
		switch {
		case blank:
			mark, markColor = " + ", theme.TextDisabled
		case enabled:
			mark, markColor = " ☑ ", theme.Success
		}
		toggle = t.Text{Content: mark, Style: t.Style{ForegroundColor: markColor}, Click: func(t.MouseEvent) { e.toggle(row.id) }}
		keyPlaceholder := ""
		if blank {
			keyPlaceholder = r.view.KeyPlaceholder
		}
		key = input{
			ID:          e.inputID(row.id, "key"),
			State:       row.key,
			Placeholder: keyPlaceholder,
			Highlighter: r.view.KeyHighlighter,
			OnChange:    onEdit,
			Keybinds:    nav("key"),
			Suggestions: row.keySuggestions,
		}
		if blank {
			remove = t.Text{Content: "  "}
		} else {
			remove = t.Text{Content: " ✕", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { e.remove(row.id) }}
		}
	}

	valuePlaceholder := ""
	if blank || e.fixedKeys {
		valuePlaceholder = r.view.ValuePlaceholder
	}
	value := input{
		ID:          e.inputID(row.id, "value"),
		State:       row.value,
		Placeholder: valuePlaceholder,
		Highlighter: r.view.ValueHighlighter,
		Width:       t.Flex(2),
		OnChange:    onEdit,
		Keybinds:    nav("value"),
		Completion:  row.values,
		Choices:     r.view.Choices,
	}

	style := t.Style{Width: t.Flex(1), Height: t.Cells(1)}
	if !enabled && !blank && !e.fixedKeys {
		style.Faint = true
	}
	return t.Row{
		Style:    style,
		Spacing:  1,
		Children: []t.Widget{toggle, key, value, remove},
	}
}
