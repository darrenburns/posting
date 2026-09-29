package ui

import (
	t "github.com/darrenburns/terma"
)

// formScroll is where a scrollForm is scrolled to.
type formScroll struct {
	scroll *t.ScrollState
	// rows is where each row landed in the last layout: its top and height.
	rows [][2]int
	// revealed is the focused ID last scrolled into view. Revealing only
	// when it changes means wheel scrolling isn't undone by the next build.
	revealed string
}

func newFormScroll() *formScroll { return &formScroll{scroll: t.NewScrollState()} }

// reveal scrolls the row holding the focused widget into view, if focus has
// moved to it since the last reveal.
func (f *formScroll) reveal(focused string, row int) {
	if focused == f.revealed {
		return
	}
	f.revealed = focused
	if row >= 0 && row < len(f.rows) {
		f.scroll.ScrollToView(f.rows[row][0], f.rows[row][1])
	}
}

// formField is one row of a scrollForm, with the IDs of the focusable
// widgets in it.
type formField struct {
	child t.Widget
	ids   []string
}

func field(child t.Widget, ids ...string) formField { return formField{child: child, ids: ids} }

// scrollForm stacks rows in a column that scrolls when the panel is too
// short to show them all. Terma doesn't scroll focus into view by itself, so
// the form rebuilds when focus moves to one of its rows and scrolls to that
// row.
//
// The scroll has to happen while building: the Scrollable reads its offset
// when it is laid out, and changes made later in the frame are dropped. The
// rows don't move when focus does, so where they landed in the last layout
// is where they are now.
type scrollForm struct {
	State   *formScroll
	Spacing int
	Rows    []formField
	// Fit sizes the form to its rows, for dialogs, instead of filling its
	// parent. It still scrolls when the screen is too short for it.
	Fit bool
}

func (f scrollForm) height() t.Dimension {
	if f.Fit {
		return t.Auto
	}
	return t.Flex(1)
}

func (f scrollForm) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), f.height() }

func (f scrollForm) Build(ctx t.BuildContext) t.Widget {
	rowOf := map[string]int{}
	children := make([]t.Widget, len(f.Rows))
	for i, row := range f.Rows {
		children[i] = row.child
		for _, id := range row.ids {
			rowOf[id] = i
		}
	}
	// Only rebuild when focus enters, leaves or moves within the form.
	focused := ""
	if signal := ctx.FocusedSignal(); signal.IsValid() {
		focused = t.SelectAny(signal, func(w t.Focusable) string {
			id := focusedIDOf(w)
			if _, ok := rowOf[id]; ok {
				return id
			}
			return ""
		})
	}
	row := -1
	if focused != "" {
		row = rowOf[focused]
	}
	f.State.reveal(focused, row)
	return t.Scrollable{
		State: f.State.scroll,
		Style: t.Style{Width: t.Flex(1), Height: f.height()},
		Child: formColumn{
			Column: t.Column{Style: t.Style{Width: t.Flex(1)}, Spacing: f.Spacing, Children: children},
			state:  f.State,
		},
	}
}

// formColumn is the scrollForm's content. It is a Column in its own right,
// rather than a widget that builds one, so it is told where its rows landed.
type formColumn struct {
	t.Column
	state *formScroll
}

func (c formColumn) Build(t.BuildContext) t.Widget { return c }
func (c formColumn) ChildWidgets() []t.Widget      { return c.Children }

func (c formColumn) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	rows := make([][2]int, metrics.ChildCount())
	for i := range rows {
		y, _ := metrics.ChildY(i)
		height, _ := metrics.ChildHeight(i)
		rows[i] = [2]int{y, height}
	}
	c.state.rows = rows
}
