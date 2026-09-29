package ui

import (
	"strings"

	t "github.com/darrenburns/terma"
)

// action is a global command with a keymap ID. The IDs are Posting 2's, so
// the keymap section of an existing config keeps working; commands Posting 2
// didn't have get new IDs in the same style.
type action struct {
	id   string
	keys []string // defaults; the keymap replaces them
	name string   // footer label
	help string   // help overlay description
	// hidden keeps it out of the footer.
	hidden bool
	run    func()
}

func (a *App) actions() []action {
	return []action{
		{id: "send-request", keys: []string{"ctrl+j", "alt+enter"}, name: "Send", help: "Send the request", run: a.send},
		{id: "jump", keys: []string{t.DefaultJumpKey}, name: "Jump", help: "Jump mode: move focus by typing a label", hidden: true, run: a.jump.Activate},
		{id: "search-requests", keys: []string{"ctrl+g"}, name: "Search requests", help: "Search the collection", hidden: true, run: a.focusTreeSearch},
		{id: "save-request", keys: []string{"ctrl+s"}, name: "Save", help: "Save the request to the collection", run: a.saveRequest},
		{id: "new-request", keys: []string{"ctrl+n"}, name: "New tab", help: "Open a new request tab", run: a.newTab},
		{id: "commands", keys: []string{"ctrl+p"}, name: "Commands", help: "Command palette", run: a.openPalette},
		{id: "close-tab", keys: []string{"alt+w"}, name: "Close tab", help: "Close the request tab", hidden: true, run: func() { a.closeSession(a.active.Peek()) }},
		// Unbound by default: editing, sending or saving a preview tab keeps
		// it too. The keymap can give it a key.
		{id: "keep-tab", name: "Keep tab", help: "Keep the preview tab open", hidden: true, run: func() { a.keepSession(a.active.Peek()) }},
		{id: "next-tab", keys: []string{"alt+right"}, name: "Next tab", help: "Next request tab", hidden: true, run: func() { a.cycleSession(1) }},
		{id: "previous-tab", keys: []string{"alt+left"}, name: "Prev tab", help: "Previous request tab", hidden: true, run: func() { a.cycleSession(-1) }},
		{id: "search-tabs", keys: []string{"alt+down"}, name: "Tabs", help: "Search the open request tabs", hidden: true, run: a.openTabSearch},
		{id: "focus-url", keys: []string{"ctrl+l"}, name: "Focus URL", help: "Focus the URL bar", hidden: true, run: func() { t.RequestFocus(urlInputID) }},
		{id: "focus-method", keys: []string{"ctrl+t"}, name: "Method", help: "Choose the HTTP method", hidden: true, run: a.openMethodMenu},
		{id: "toggle-collection", keys: []string{"ctrl+h"}, name: "Sidebar", help: "Show or hide the collection", hidden: true, run: a.toggleSidebar},
		{id: "expand-section", keys: []string{"alt+z"}, name: "Expand", help: "Expand the focused panel", hidden: true, run: func() { a.toggleExpand("request") }},
		{id: "variables", keys: []string{"ctrl+shift+v"}, name: "Variables", help: "Variables", hidden: true, run: a.openVariables},
		{id: "help", keys: []string{"f1"}, name: "Help", help: "This help", run: func() { a.overlay.Set("help") }},
	}
}

// keysFor is the keys bound to an action after applying the keymap.
func (a *App) keysFor(id string) []string {
	for _, act := range a.actions() {
		if act.id == id {
			return a.settings.KeysFor(id, act.keys...)
		}
	}
	return nil
}

// keyHint is the first key bound to an action, for hints in menus.
func (a *App) keyHint(id string) string {
	if keys := a.keysFor(id); len(keys) > 0 {
		return keys[0]
	}
	return ""
}

// jumpKey is the key that toggles jump mode.
func (a *App) jumpKey() string { return a.keyHint("jump") }

// Keybinds are the global shortcuts. Widgets see keys first, so text inputs
// keep their editing keys and these only fire when nothing else claims them.
func (a *App) Keybinds() []t.Keybind {
	binds := []t.Keybind{{Key: "escape", Name: "Cancel", Action: a.cancelSend, Hidden: true}}
	for _, act := range a.actions() {
		if act.id == "jump" {
			continue // Bound by the Jumper.
		}
		for i, key := range a.settings.KeysFor(act.id, act.keys...) {
			binds = append(binds, t.Keybind{Key: key, Name: act.name, Action: act.run, Hidden: act.hidden || i > 0})
		}
	}
	return binds
}

// helpForActions lists the global shortcuts for the help overlay.
func (a *App) helpForActions() [][2]string {
	var rows [][2]string
	for _, act := range a.actions() {
		keys := a.settings.KeysFor(act.id, act.keys...)
		if len(keys) == 0 {
			continue
		}
		rows = append(rows, [2]string{strings.Join(keys, " / "), act.help})
	}
	return append(rows,
		[2]string{"esc", "Cancel an in-flight request"},
		[2]string{"ctrl+c", "Quit"},
	)
}
