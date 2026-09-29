package ui

import (
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// methodColor is the colour used for a method everywhere it appears.
func methodColor(theme t.ThemeData, m model.Method) t.Color {
	switch m {
	case model.MethodGet:
		return theme.PrimaryText
	case model.MethodPost:
		return theme.SuccessText
	case model.MethodPut:
		return theme.WarningText
	case model.MethodPatch:
		return theme.InfoText
	case model.MethodDelete:
		return theme.ErrorText
	default:
		return theme.TextMuted
	}
}

// statusColors returns the foreground and background for a status code chip.
func statusColors(theme t.ThemeData, code int) (fg, bg t.Color) {
	switch model.ClassifyStatus(code) {
	case model.StatusClassSuccess:
		return theme.SuccessText, theme.SuccessBg
	case model.StatusClassRedirect:
		return theme.WarningText, theme.WarningBg
	default:
		return theme.ErrorText, theme.ErrorBg
	}
}

// focusedID returns the ID of the focused widget without subscribing to focus.
func focusedIDOf(f t.Focusable) string {
	if f == nil {
		return ""
	}
	if identifiable, ok := f.(t.Identifiable); ok {
		return identifiable.WidgetID()
	}
	return ""
}

// focusWithin reports whether the focused widget's ID starts with prefix.
// The caller only rebuilds when the answer changes, not on every focus move.
func focusWithin(ctx t.BuildContext, prefix string) bool {
	signal := ctx.FocusedSignal()
	if !signal.IsValid() {
		return false
	}
	return t.SelectAny(signal, func(f t.Focusable) bool {
		return strings.HasPrefix(focusedIDOf(f), prefix)
	})
}

// isFocusedID reports whether the widget with the given ID is focused.
func isFocusedID(ctx t.BuildContext, id string) bool {
	signal := ctx.FocusedSignal()
	if !signal.IsValid() {
		return false
	}
	return t.SelectAny(signal, func(f t.Focusable) bool { return focusedIDOf(f) == id })
}

// Terma sizes a child from the widget value its parent returned, before that
// child is built. Composite widgets therefore have to declare how they want to
// be sized themselves; what their Build returns is not consulted. Embed one of
// these in composites that should stretch.

// fillParent makes a composite widget expand in both directions.
type fillParent struct{}

func (fillParent) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), t.Flex(1) }

// fillWidth makes a composite widget expand horizontally only.
type fillWidth struct{}

func (fillWidth) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), t.Auto }

// inset indents plain text by one cell. Boxed widgets (inputs, text areas,
// chips) start at a panel's content edge and pad their text by one cell;
// plain text beside or below them is inset to line up with that text, and
// with the tab labels above.
var inset = t.EdgeInsetsTRBL(0, 0, 0, 1)

// section is a titled panel drawn straight onto the background. Its heading
// brightens while focus is anywhere inside it, which is detected through the
// shared ID prefix.
type section struct {
	ID        string
	Prefix    string // focus-within prefix for the children's IDs
	Title     string
	TitleMark string // markup appended to the title (e.g. a status chip)
	Subtitle  string // right-aligned markup on the heading row
	Child     t.Widget
	Width     t.Dimension
	Height    t.Dimension
}

func (s section) GetContentDimensions() (t.Dimension, t.Dimension) { return s.Width, s.Height }

func (s section) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	title := "[b $TextMuted]" + s.Title + "[/]"
	if focusWithin(ctx, s.Prefix) {
		title = "[b $Text]" + s.Title + "[/]"
	}
	heading := []t.Widget{t.ParseMarkupToText(title+s.TitleMark, theme), t.Spacer{Width: t.Flex(1)}}
	if s.Subtitle != "" {
		heading = append(heading, t.ParseMarkupToText(s.Subtitle, theme))
	}
	return t.Column{
		ID: s.ID,
		Style: t.Style{
			Width:   s.Width,
			Height:  s.Height,
			Padding: t.EdgeInsets{Right: 1},
		},
		Children: []t.Widget{
			t.Row{Style: t.Style{Width: t.Flex(1), Padding: inset}, Children: heading},
			s.Child,
		},
	}
}

// emptyState is a centred, muted message for panels with nothing to show.
type emptyState struct {
	fillParent
	Title string
	Lines []string
}

func (e emptyState) Build(ctx t.BuildContext) t.Widget {
	children := []t.Widget{
		t.Text{Content: e.Title, TextAlign: t.TextAlignCenter, Style: t.Style{ForegroundColor: ctx.Theme().Text, Bold: true, Width: t.Flex(1)}},
	}
	for _, line := range e.Lines {
		text := t.ParseMarkupToText(line, ctx.Theme())
		text.TextAlign = t.TextAlignCenter
		text.Wrap = t.WrapSoft
		text.Style.ForegroundColor = ctx.Theme().TextMuted
		text.Style.Width = t.Flex(1)
		children = append(children, text)
	}
	return t.Column{
		Style:      t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		MainAlign:  t.MainAxisCenter,
		CrossAlign: t.CrossAxisCenter,
		Children:   children,
	}
}

// fieldLabel renders a form label with an optional muted suffix such as "optional".
func fieldLabel(ctx t.BuildContext, label, suffix string) t.Widget {
	markup := "[b]" + label + "[/]"
	if suffix != "" {
		markup += " [i $TextMuted]" + suffix + "[/]"
	}
	return t.ParseMarkupToText(markup, ctx.Theme())
}

// inputStyle is the shared look for text inputs. Inputs sit beside a
// one-cell focus bar that stands in for their left padding, so their text
// lines up with text areas (padded by one cell) in the same form.
func inputStyle(theme t.ThemeData, focused bool) t.Style {
	style := t.Style{
		Width:           t.Flex(1),
		BackgroundColor: theme.Surface,
		ForegroundColor: theme.Text,
		Padding:         t.EdgeInsetsTRBL(0, 1, 0, 0),
	}
	if focused {
		style.BackgroundColor = theme.Surface2
	}
	return style
}

// input wraps a TextInput with a focus bar on its left edge so the focused
// field is obvious even in dense forms.
type input struct {
	ID          string
	State       *t.TextInputState
	Placeholder string
	Highlighter t.Highlighter
	Width       t.Dimension
	OnChange    func(string)
	OnSubmit    func(string)
	Keybinds    []t.Keybind
	// Suggestions, when set, shows a completion popup while typing.
	Suggestions *t.AutocompleteState
	// DisableFocus takes the input out of the focus order.
	DisableFocus bool
	// Completion, when set, completes ${VARIABLES} from Choices.
	Completion *completion
	Choices    variableChoices
}

func (i input) GetContentDimensions() (t.Dimension, t.Dimension) {
	if i.Width.IsUnset() {
		return t.Flex(1), t.Cells(1)
	}
	return i.Width, t.Cells(1)
}

func (i input) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	focused := isFocusedID(ctx, i.ID)
	bar := theme.Surface
	if focused {
		bar = theme.Accent
	}
	width := i.Width
	if width.IsUnset() {
		width = t.Flex(1)
	}
	style := inputStyle(theme, focused)
	style.Width = t.Flex(1)
	var field t.Widget = t.TextInput{
		ID:            i.ID,
		State:         i.State,
		Placeholder:   i.Placeholder,
		Highlighter:   i.Highlighter,
		Style:         style,
		OnChange:      i.OnChange,
		OnSubmit:      i.OnSubmit,
		ExtraKeybinds: i.Keybinds,
		DisableFocus:  i.DisableFocus,
	}
	if i.Completion != nil {
		field = i.Completion.wrap(theme, field, i.Choices, t.Flex(1))
	}
	if i.Suggestions != nil {
		field = t.Autocomplete{
			State:                 i.Suggestions,
			Child:                 field,
			Width:                 t.Flex(1),
			MatchMode:             t.FilterFuzzy,
			Insert:                t.InsertReplace,
			MinChars:              1,
			MaxVisible:            8,
			DismissWhenEmpty:      true,
			DisableKeysWhenHidden: true,
			PopupWidth:            t.Cells(64),
			PopupStyle:            t.Style{BackgroundColor: theme.Surface2},
			RenderSuggestion:      renderSuggestion,
			OnSelect: func(t.Suggestion) {
				if i.OnChange != nil {
					i.OnChange(i.State.GetText())
				}
			},
		}
	}
	return t.Row{
		Style: t.Style{Width: width, Height: t.Cells(1)},
		Children: []t.Widget{
			t.Text{Content: "▎", Style: t.Style{ForegroundColor: bar, BackgroundColor: theme.Surface}},
			field,
		},
	}
}

// renderSuggestion draws a completion with its description dimmed beside it.
func renderSuggestion(s t.Suggestion, active bool, match t.MatchResult, ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	fg, bg, muted := theme.Text, theme.Surface2, theme.TextMuted
	if active {
		fg, bg, muted = theme.SelectionText, theme.ActiveCursor, theme.SelectionText
	}
	label := []t.Span{{Text: s.Label, Style: t.SpanStyle{Foreground: fg}}}
	if match.Matched {
		label = t.HighlightSpans(s.Label, match.Ranges, t.MatchHighlightStyle(theme))
	}
	children := []t.Widget{t.Text{Spans: label, Style: t.Style{ForegroundColor: fg}}}
	if s.Description != "" {
		children = append(children, t.Spacer{Width: t.Flex(1)}, t.Text{Content: "  " + s.Description, Style: t.Style{ForegroundColor: muted}})
	}
	return t.Row{Style: t.Style{Width: t.Flex(1), BackgroundColor: bg, Padding: t.EdgeInsetsXY(1, 0)}, Children: children}
}

// isBlank reports reactively whether a text input is empty. It only
// re-triggers when emptiness flips, not on every keystroke.
func isBlank(state *t.TextInputState) bool {
	return t.SelectAny(state.Content, func(graphemes []string) bool { return len(graphemes) == 0 })
}

// textOf reads a text input's content reactively.
func textOf(state *t.TextInputState) string {
	return strings.Join(state.Content.Get(), "")
}

// scrollingArea puts a text area (or a completion wrapping one) in a
// Scrollable that shares the area's ScrollState. That is how Terma text
// areas scroll: the view follows the cursor, a scrollbar shows where you
// are, and the mouse wheel works. The area itself should have no height so
// it grows with its text; clicking the space below the text focuses it.
func scrollingArea(id string, scroll *t.ScrollState, background t.Color, area t.Widget) t.Widget {
	return t.Scrollable{
		State: scroll,
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: background},
		Child: area,
		Click: func(t.MouseEvent) { t.RequestFocus(id) },
	}
}
