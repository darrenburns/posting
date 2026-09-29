package ui

import (
	"unicode/utf8"

	t "github.com/darrenburns/terma"
)

const (
	sessionTabsWheelID = "session-tabs-wheel"
	tabSearchID        = "tab-search"
)

// The fixed parts of the strip of open requests, in cells.
const (
	newTabWidth  = 3 // " + "
	tabListWidth = 3 // " ▾ "
	// tabSearchWidth is the width of the open-tab search dropdown.
	tabSearchWidth = 56
)

// sessionTabsView is where the strip of open requests is scrolled to.
type sessionTabsView struct {
	first int // Index of the first tab shown
	// maxFirst is the furthest the strip can scroll, as of its last build.
	maxFirst int
	// revealed is the session last scrolled into view. When the active
	// session differs, the strip scrolls to it.
	revealed int
	// x, y and width are where the strip was last painted. Layout doesn't
	// tell widgets their size while they build, so a probe records it.
	x, y, width int
	// changed rebuilds the strip after it scrolls or is resized.
	changed t.Signal[int]
	// wheel receives mouse wheel events over a strip that overflows.
	wheel *t.ScrollState
}

func newSessionTabsView() *sessionTabsView {
	v := &sessionTabsView{changed: t.NewSignal(0), wheel: t.NewScrollState()}
	// Terma only sends the wheel to scrollables, and only while they can
	// scroll, so the strip is wrapped in one (see sessionTabs.Build). Every
	// direction moves it a tab at a time; returning true keeps the
	// scrollable's own offset where it is.
	v.wheel.OnScrollUp = func(int) bool { v.scroll(-1); return true }
	v.wheel.OnScrollLeft = func(int) bool { v.scroll(-1); return true }
	v.wheel.OnScrollDown = func(int) bool { v.scroll(1); return true }
	v.wheel.OnScrollRight = func(int) bool { v.scroll(1); return true }
	return v
}

// scroll moves the strip by delta tabs, leaving the active tab where it is.
func (v *sessionTabsView) scroll(delta int) {
	first := min(max(v.first+delta, 0), v.maxFirst)
	if first != v.first {
		v.first = first
		v.refresh()
	}
}

// revealActive scrolls the active tab into view on the next build, even if
// it was the last one revealed and has since been scrolled away from.
func (v *sessionTabsView) revealActive() {
	v.revealed = 0
	v.refresh()
}

func (v *sessionTabsView) refresh() { v.changed.Set(v.changed.Peek() + 1) }

// sessionTabsFit measures tabs against the room the strip has for them.
type sessionTabsFit struct {
	widths []int
	room   int
}

// fits reports whether tabs first..last fit, with room for the overflow
// marks either side of them.
func (f sessionTabsFit) fits(first, last int) bool {
	used := 0
	if first > 0 {
		used += overflowMarkWidth
	}
	if last < len(f.widths)-1 {
		used += overflowMarkWidth
	}
	for _, w := range f.widths[first : last+1] {
		used += w
	}
	return used <= f.room
}

// maxFirst is the first tab shown when scrolled all the way along: scrolling
// further would only leave space after the last tab.
func (f sessionTabsFit) maxFirst() int {
	for first := range f.widths {
		if f.fits(first, len(f.widths)-1) {
			return first
		}
	}
	return max(len(f.widths)-1, 0)
}

// reveal scrolls from first as little as it can to bring tab index into
// view.
func (f sessionTabsFit) reveal(first, index int) int {
	first = min(first, index)
	for first < index && !f.fits(first, index) {
		first++
	}
	return first
}

// sessionTabs is the strip of open requests above the URL bar. When there
// are more than fit, it scrolls sideways a tab at a time, with ‹ and › marks
// at the edges that hide tabs, and keeps the active tab in view. With more
// than one tab open, ▾ at the right end searches them.
type sessionTabs struct {
	fillWidth
	app *App
}

func (st sessionTabs) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := st.app
	view := a.sessionView
	view.changed.Get()
	active := a.active.Get()
	sessions := a.sessions.Get()

	chips := make([]t.Widget, len(sessions))
	fit := sessionTabsFit{widths: make([]int, len(sessions)), room: view.width - newTabWidth}
	total, activeIndex := 0, 0
	for i, s := range sessions {
		chip := sessionChip{app: a, session: s, active: s.id == active, closable: len(sessions) > 1 || !s.isPristine()}
		chips[i] = chip
		fit.widths[i] = chip.width()
		total += fit.widths[i]
		if s.id == active {
			activeIndex = i
		}
	}
	listed := len(sessions) > 1
	if listed {
		fit.room -= tabListWidth
	}
	newTab := t.Text{
		Content: " + ",
		Style:   t.Style{ForegroundColor: theme.TextMuted},
		Click:   func(t.MouseEvent) { a.newTab() },
	}
	tabList := t.Text{
		ID:      tabSearchID + "-button",
		Content: " ▾ ",
		Style:   t.Style{ForegroundColor: theme.TextMuted},
		Click:   func(t.MouseEvent) { a.openTabSearch() },
	}

	overflow := view.width > 0 && total > fit.room
	if !overflow {
		view.first, view.maxFirst = 0, 0
		if view.width > 0 {
			view.revealed = active
		}
		children := append(chips, newTab)
		if listed {
			children = append(children, t.Spacer{}, tabList)
		}
		return st.probed(t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: children})
	}

	// The scrollable the tabs sit in keeps a column for its scrollbar.
	fit.room--
	view.maxFirst = fit.maxFirst()
	view.first = min(view.first, view.maxFirst)
	if view.revealed != active {
		view.first = fit.reveal(view.first, activeIndex)
		view.revealed = active
	}
	first := view.first

	shown := make([]t.Widget, 0, len(chips)-first+1)
	used := 0
	if first > 0 {
		shown = append(shown, overflowMarkText(theme, "‹ ", func() { view.scroll(-1) }))
		used += overflowMarkWidth
	}
	shown = append(shown, chips[first:]...)
	for _, w := range fit.widths[first:] {
		used += w
	}
	// Tabs past the right edge are clipped; the › mark says there are more.
	tabs := t.Scrollable{
		ID:                  sessionTabsWheelID,
		State:               view.wheel,
		DisableFocus:        true,
		Style:               t.Style{Width: t.Flex(1), Height: t.Cells(1)},
		ScrollbarThumbColor: theme.Background,
		ScrollbarTrackColor: theme.Background,
		// The blank second row lets the scrollable scroll vertically, so the
		// wheel reaches it; the scrollable is one row tall and never moves.
		Child: t.Column{
			Style: t.Style{Width: t.Flex(1)},
			Children: []t.Widget{
				t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: shown},
				t.Text{Content: " "},
			},
		},
	}
	children := []t.Widget{tabs}
	if used > fit.room {
		children = append(children, overflowMarkText(theme, " ›", func() { view.scroll(1) }))
	}
	children = append(children, newTab)
	if listed {
		children = append(children, tabList)
	}
	return st.probed(t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: children})
}

// probed lays a probe under the strip to measure it.
func (st sessionTabs) probed(row t.Widget) t.Widget {
	return t.Stack{
		Style:    t.Style{Width: t.Flex(1), Height: t.Cells(1)},
		Children: []t.Widget{sessionTabsProbe{view: st.app.sessionView}, row},
	}
}

// sessionTabsProbe fills the strip and records where it is painted, so the
// strip knows how many tabs fit and the tab search can drop down from it.
type sessionTabsProbe struct{ view *sessionTabsView }

func (p sessionTabsProbe) GetContentDimensions() (t.Dimension, t.Dimension) {
	return t.Flex(1), t.Cells(1)
}

func (p sessionTabsProbe) Build(t.BuildContext) t.Widget { return p }

// Render records the strip's position. A change of width rebuilds the strip
// in the next frame, since this one has already been built for the old one,
// scrolled to keep the active tab in view.
func (p sessionTabsProbe) Render(ctx *t.RenderContext) {
	v := p.view
	v.x, v.y = ctx.X, ctx.Y
	if ctx.Width != v.width {
		v.width = ctx.Width
		t.Dispatch(v.revealActive)
	}
}

type sessionChip struct {
	app      *App
	session  *Session
	active   bool
	closable bool
}

// Jump switches to this request tab, for jump mode (see t.Jumpable).
func (c sessionChip) Jump() { c.app.active.Set(c.session.id) }

// closer is the chip's close button, or the padding in its place.
func (c sessionChip) closer() string {
	if c.closable {
		return " ✕ "
	}
	return " "
}

// width is the number of cells the chip takes up, as Build draws it.
func (c sessionChip) width() int {
	s := c.session
	text := " " + s.method.Get().Short() + " " + truncate(s.title.Get(), 28)
	if s.phase.Get() == exchangeSending {
		text += " …"
	}
	return utf8.RuneCountInString(text + c.marker() + c.closer())
}

// marker shows that the request has unsaved changes.
func (c sessionChip) marker() string {
	if c.session.dirty.Get() {
		return " ●"
	}
	return " "
}

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
	spans = append(spans, t.Span{Text: c.marker(), Style: t.SpanStyle{Foreground: theme.WarningText, Background: bg}})
	id := s.id
	children := []t.Widget{t.Text{Spans: spans, Click: func(t.MouseEvent) { c.app.active.Set(id) }}}
	if c.closable {
		children = append(children, t.Text{
			Content: c.closer(),
			Style:   t.Style{ForegroundColor: theme.TextMuted, BackgroundColor: bg},
			Click:   func(t.MouseEvent) { c.app.closeSession(id) },
		})
	} else {
		children = append(children, t.Text{Content: c.closer(), Style: t.Style{BackgroundColor: bg}})
	}
	return t.Row{Children: children}
}

// ---------------------------------------------------------------------------
// Searching the open tabs

// openTabSearch lists the open request tabs in a dropdown under the strip,
// starting on the active one.
func (a *App) openTabSearch() {
	active := a.active.Peek()
	sessions := a.sessions.Peek()
	items := make([]t.CommandPaletteItem, 0, len(sessions))
	for _, s := range sessions {
		id := s.id
		items = append(items, t.CommandPaletteItem{
			Label:   s.title.Peek(),
			Hint:    s.file.Peek(),
			Current: id == active,
			Data:    s,
			Action: func() {
				a.tabSearch.Close(false)
				a.showSession(id)
			},
		})
	}
	a.tabSearch.SetItems(items)
	a.tabSearch.Open()
}

// showSession switches to a session and scrolls its tab into view.
func (a *App) showSession(id int) {
	a.active.Set(id)
	a.sessionView.revealActive()
}

// tabSearchPalette is the command palette, dropped down from the right end of
// the strip of open tabs rather than floating over the middle of the screen.
func (a *App) tabSearchPalette(theme t.ThemeData) t.Widget {
	view := a.sessionView
	return t.CommandPalette{
		ID:          tabSearchID,
		State:       a.tabSearch,
		Placeholder: "Search open tabs…",
		Position:    t.FloatPositionAbsolute,
		Offset:      t.Offset{X: max(view.x+view.width-tabSearchWidth, 0), Y: view.y + 1},
		Style:       t.Style{Width: t.Cells(tabSearchWidth)},
		RenderItem: func(item t.CommandPaletteItem, active bool, match t.MatchResult) t.Widget {
			s, _ := item.Data.(*Session)
			if s == nil {
				return t.EmptyWidget{}
			}
			marker := ""
			if s.dirty.Peek() {
				marker = "●"
			}
			method := s.method.Peek()
			return renderMethodItem(theme, item, active, match, method, padRight(method.Short(), 4), marker)
		},
	}
}
