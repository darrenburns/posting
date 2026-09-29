package ui

import (
	"strconv"
	"unicode/utf8"

	t "github.com/darrenburns/terma"
)

// tabItem is one entry in a tabStrip.
type tabItem struct {
	Key   string
	Label string
	// Badge is a short count shown after the label, such as "2".
	Badge string
	// Marked adds a dot after the label to show the tab has content.
	Marked bool
}

// tabStrip is a focusable row of tabs. It only switches the active key; the
// caller renders the matching content (usually with a Switcher).
type tabStrip struct {
	ID       string
	Tabs     []tabItem
	Active   t.Signal[string]
	Disabled bool
	OnChange func(key string)
	// View, when set, lets the strip scroll in a panel too narrow for every
	// tab, keeping the selected one in view.
	View *tabView
	// Down and Up move focus into the tab's content and back out of the
	// strip, so the keyboard can travel through the layout.
	Down func()
	Up   func()
}

// tabView is where a strip too narrow for its tabs is scrolled to.
type tabView struct {
	first t.Signal[int] // Index of the first tab shown
	// width is the strip's width as of its last paint. Layout doesn't tell
	// widgets their size while they build, so a probe records it.
	width int
}

func newTabView() *tabView { return &tabView{first: t.NewSignal(0)} }

// overflowMarkWidth is the width of the ‹ and › marks shown when tabs are
// scrolled out of view.
const overflowMarkWidth = 2

func (s tabStrip) GetContentDimensions() (t.Dimension, t.Dimension) {
	if s.View != nil {
		return t.Flex(1), t.Cells(1)
	}
	return t.Auto, t.Cells(1)
}

func (s tabStrip) WidgetID() string            { return s.ID }
func (s tabStrip) IsFocusable() bool           { return !s.Disabled }
func (s tabStrip) OnKey(event t.KeyEvent) bool { return false }

func (s tabStrip) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "left", Name: "Prev tab", Action: func() { s.step(-1) }, Hidden: true},
		{Key: "h", Name: "Prev tab", Action: func() { s.step(-1) }, Hidden: true},
		{Key: "right", Name: "Next tab", Action: func() { s.step(1) }, Hidden: true},
		{Key: "l", Name: "Next tab", Action: func() { s.step(1) }, Hidden: true},
		{Key: "down", Name: "Into tab", Action: s.down, Hidden: true},
		{Key: "j", Name: "Into tab", Action: s.down, Hidden: true},
		{Key: "enter", Name: "Into tab", Action: s.down, Hidden: true},
		{Key: "up", Name: "Out", Action: s.up, Hidden: true},
		{Key: "k", Name: "Out", Action: s.up, Hidden: true},
	}
}

func (s tabStrip) down() {
	if s.Down != nil {
		s.Down()
	}
}

func (s tabStrip) up() {
	if s.Up != nil {
		s.Up()
	}
}

func (s tabStrip) indexOf(key string) int {
	for i, tab := range s.Tabs {
		if tab.Key == key {
			return i
		}
	}
	return 0
}

func (s tabStrip) step(delta int) {
	if len(s.Tabs) == 0 {
		return
	}
	index := (s.indexOf(s.Active.Peek()) + delta + len(s.Tabs)) % len(s.Tabs)
	s.selectKey(s.Tabs[index].Key)
}

func (s tabStrip) selectKey(key string) {
	if s.Disabled {
		return
	}
	s.Active.Set(key)
	s.reveal(key)
	if s.OnChange != nil {
		s.OnChange(key)
	}
}

// width is the number of cells a tab takes up, padding included.
func (tab tabItem) width() int {
	w := 2 + utf8.RuneCountInString(tab.Label)
	switch {
	case tab.Badge != "":
		w += 1 + utf8.RuneCountInString(tab.Badge)
	case tab.Marked:
		w++
	}
	return w
}

// fits reports whether tabs first..last fit in the strip, with room for the
// overflow marks either side of them.
func (s tabStrip) fits(first, last int) bool {
	used := 0
	if first > 0 {
		used += overflowMarkWidth
	}
	if last < len(s.Tabs)-1 {
		used += overflowMarkWidth
	}
	for _, tab := range s.Tabs[first : last+1] {
		used += tab.width()
	}
	return used <= s.View.width
}

// reveal scrolls the strip as little as it can to bring the tab with key into
// view.
func (s tabStrip) reveal(key string) {
	if s.View == nil || s.View.width == 0 {
		return
	}
	index := s.indexOf(key)
	first := min(s.View.first.Peek(), index)
	if s.fits(0, len(s.Tabs)-1) {
		first = 0
	}
	for first < index && !s.fits(first, index) {
		first++
	}
	s.View.first.Set(first)
}

// countBadge is a tab badge for n items, or nothing when there are none.
func countBadge(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// tabID is the ID of one tab's label, which jump mode targets.
func tabID(stripID, key string) string { return stripID + "-" + key }

// Build draws each tab as " Label " so neighbours are two cells apart. Only
// the label itself is underlined, never the padding around it.
func (s tabStrip) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	active := s.Active.Get()
	focused := ctx.IsFocused(s)
	first := 0
	if s.View != nil && len(s.Tabs) > 0 {
		first = min(s.View.first.Get(), len(s.Tabs)-1)
		if s.View.width > 0 && s.fits(0, len(s.Tabs)-1) {
			first = 0 // Widened since it last scrolled.
		}
	}
	children := make([]t.Widget, 0, len(s.Tabs)+2)
	if first > 0 {
		children = append(children, s.overflowMark(theme, "‹ ", -1))
	}
	width := 0
	for _, tab := range s.Tabs[first:] {
		key := tab.Key
		width += tab.width()
		var bg t.Color
		label := t.SpanStyle{Foreground: theme.TextMuted}
		extra := t.SpanStyle{Foreground: theme.TextMuted}
		switch {
		case s.Disabled:
			label.Foreground = theme.TextDisabled
		case key == active && focused:
			bg = theme.ActiveCursor
			label = t.SpanStyle{Foreground: theme.SelectionText, Bold: true}
			extra.Foreground = theme.SelectionText
		case key == active:
			label = t.SpanStyle{Foreground: theme.Text, Bold: true, Underline: t.UnderlineSingle, UnderlineColor: theme.Accent}
			extra.Foreground = theme.AccentText
		}
		label.Background, extra.Background = bg, bg
		pad := t.Span{Text: " ", Style: t.SpanStyle{Background: bg}}
		spans := []t.Span{pad, {Text: tab.Label, Style: label}}
		switch {
		case tab.Badge != "":
			spans = append(spans, t.Span{Text: " " + tab.Badge, Style: extra})
		case tab.Marked:
			if !bg.IsSet() {
				extra.Foreground = theme.AccentText
			}
			spans = append(spans, t.Span{Text: "•", Style: extra})
		}
		spans = append(spans, pad)
		children = append(children, tabLabel{
			id:       tabID(s.ID, key),
			spans:    spans,
			activate: func() { s.selectKey(key) },
		})
	}
	if s.View == nil {
		return t.Row{Style: t.Style{Height: t.Cells(1)}, Children: children}
	}

	// Tabs past the right edge are clipped; the › mark says there are more.
	tabs := t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: children}
	row := t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: []t.Widget{tabs}}
	if first > 0 {
		width += overflowMarkWidth
	}
	if s.View.width > 0 && width > s.View.width {
		row.Children = append(row.Children, s.overflowMark(theme, " ›", 1))
	}
	return t.Stack{
		Style:    t.Style{Width: t.Flex(1), Height: t.Cells(1)},
		Children: []t.Widget{widthProbe{view: s.View}, row},
	}
}

// overflowMark is the ‹ or › shown when tabs are scrolled out of view.
// Clicking it moves to the next tab in that direction.
func (s tabStrip) overflowMark(theme t.ThemeData, mark string, delta int) t.Widget {
	return overflowMarkText(theme, mark, func() { s.step(delta) })
}

// overflowMarkText draws a ‹ or › mark for a strip scrolled sideways, so
// every strip that scrolls marks it the same way.
func overflowMarkText(theme t.ThemeData, mark string, onClick func()) t.Widget {
	return t.Text{
		Content: mark,
		Style:   t.Style{ForegroundColor: theme.AccentText, Bold: true},
		Click:   func(t.MouseEvent) { onClick() },
	}
}

// widthProbe fills its parent and records the width it is painted at.
type widthProbe struct{ view *tabView }

func (p widthProbe) GetContentDimensions() (t.Dimension, t.Dimension) {
	return t.Flex(1), t.Cells(1)
}

func (p widthProbe) Build(ctx t.BuildContext) t.Widget { return p }
func (p widthProbe) Render(ctx *t.RenderContext)       { p.view.width = ctx.Width }

// tabLabel is one clickable tab. Its ID lets the jump map label it.
type tabLabel struct {
	id       string
	spans    []t.Span
	activate func()
}

func (l tabLabel) WidgetID() string { return l.id }

// OnClick activates the tab. Clicks go to the widget as written, not to the
// Text it builds, so a Click set on that Text would never fire.
func (l tabLabel) OnClick(t.MouseEvent) { l.activate() }

func (l tabLabel) Build(ctx t.BuildContext) t.Widget {
	return t.Text{Spans: l.spans}
}

// choice is one option in a segmented control.
type choice struct {
	Value string
	Label string
}

// segmented is a focusable single-choice control rendered as joined chips,
// used instead of dropdowns for small option sets (body type, auth type...).
type segmented struct {
	ID       string
	Options  []choice
	Selected string
	OnChange func(value string)
}

func (s segmented) WidgetID() string            { return s.ID }
func (s segmented) IsFocusable() bool           { return true }
func (s segmented) OnKey(event t.KeyEvent) bool { return false }

func (s segmented) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "left", Name: "Previous", Action: func() { s.step(-1) }, Hidden: true},
		{Key: "h", Name: "Previous", Action: func() { s.step(-1) }, Hidden: true},
		{Key: "right", Name: "Next", Action: func() { s.step(1) }, Hidden: true},
		{Key: "l", Name: "Next", Action: func() { s.step(1) }, Hidden: true},
	}
}

func (s segmented) step(delta int) {
	if len(s.Options) == 0 || s.OnChange == nil {
		return
	}
	index := 0
	for i, option := range s.Options {
		if option.Value == s.Selected {
			index = i
		}
	}
	index = (index + delta + len(s.Options)) % len(s.Options)
	s.OnChange(s.Options[index].Value)
}

func (s segmented) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	focused := ctx.IsFocused(s)
	children := make([]t.Widget, 0, len(s.Options))
	for _, option := range s.Options {
		value := option.Value
		style := t.Style{ForegroundColor: theme.TextMuted, BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)}
		if value == s.Selected {
			style.ForegroundColor = theme.TextOnPrimary
			style.BackgroundColor = theme.Primary
			style.Bold = true
			if focused {
				style.BackgroundColor = theme.Accent
				style.ForegroundColor = theme.TextOnAccent
			}
		}
		children = append(children, t.Text{
			Content: option.Label,
			Style:   style,
			Click: func(t.MouseEvent) {
				if s.OnChange != nil {
					s.OnChange(value)
				}
			},
		})
	}
	return t.Row{Style: t.Style{Height: t.Cells(1)}, Children: children}
}
