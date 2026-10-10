package ui

import (
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

// The response's headers, cookies and trailers tables copy the row under the
// cursor without Posting 2's copy menu: y (or c) copies the value, as in the
// response body, and the menu's own letters n and b the name and both.

// tableRow is a response table row as its copy keys see it.
type tableRow struct {
	kind  string // "header", "cookie" or "trailer"
	name  string
	value string
	// line is the whole row as a request would carry it: "Name: value" for
	// a header, "name=value" for a cookie.
	line string
}

func headerRow(kind string, h model.Header) tableRow {
	return tableRow{kind: kind, name: h.Name, value: h.Value, line: h.Name + ": " + h.Value}
}

func cookieRow(c model.Cookie) tableRow {
	return tableRow{kind: "cookie", name: c.Name, value: c.Value, line: c.Name + "=" + c.Value}
}

// rowCopy is one way to copy a row. The first key is shown in the footer.
type rowCopy struct {
	keys   []string
	label  string // footer label
	action string // command palette label, after "Copy <kind> "
	text   func(tableRow) string
	toast  func(tableRow) string
}

var rowCopies = []rowCopy{
	{[]string{"y", "c"}, "Copy value", "value", func(r tableRow) string { return r.value }, func(r tableRow) string { return "Copied the value of " + r.name }},
	{[]string{"n"}, "Copy name", "name", func(r tableRow) string { return r.name }, func(r tableRow) string { return "Copied " + r.name }},
	{[]string{"b"}, "Copy both", "name and value", func(r tableRow) string { return r.line }, func(r tableRow) string { return "Copied the " + r.name + " " + r.kind }},
}

func (a *App) copyRow(row tableRow, how rowCopy) {
	t.SetClipboard(t.SystemClipboard, how.text(row))
	a.notify(how.toast(row), toastSuccess)
}

// rowCopyKeybinds are the keys of s's response tables, which copy the row
// under the cursor.
func (a *App) rowCopyKeybinds(s *Session) []t.Keybind {
	var binds []t.Keybind
	for _, how := range rowCopies {
		for i, key := range how.keys {
			binds = append(binds, t.Keybind{Key: key, Name: how.label, Hidden: i > 0, Action: func() {
				if row, ok := s.cursorRow(); ok {
					a.copyRow(row, how)
				}
			}})
		}
	}
	return binds
}

// cursorRow is the row under the cursor of the response tab on show, if
// that tab is a table.
func (s *Session) cursorRow() (tableRow, bool) {
	if s.response.Peek() == nil {
		return tableRow{}, false
	}
	switch s.responseTab.Peek() {
	case "headers":
		h, ok := s.responseHeaders.SelectedRow()
		return headerRow("header", h), ok
	case "trailers":
		h, ok := s.responseTrailers.SelectedRow()
		return headerRow("trailer", h), ok
	case "cookies":
		c, ok := s.responseCookies.SelectedRow()
		return cookieRow(c), ok
	}
	return tableRow{}, false
}

// rowCopyCommands are the palette's commands for copying the row under the
// cursor of the response tab on show.
func (a *App) rowCopyCommands() []t.CommandPaletteItem {
	s := a.current()
	if s == nil {
		return nil
	}
	row, ok := s.cursorRow()
	if !ok {
		return nil
	}
	var items []t.CommandPaletteItem
	for _, how := range rowCopies {
		items = append(items, t.CommandPaletteItem{
			Label:       "Copy " + row.kind + " " + how.action,
			Description: row.name,
			Action:      a.run(func() { a.copyRow(row, how) }),
		})
	}
	return items
}
