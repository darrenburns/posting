package ui

import (
	"os"
	"os/exec"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/curl"
)

// runOnText writes text to a temporary file, runs command on it with the
// terminal handed over, and returns the file's contents once it exits. The
// file's suffix is the language, so editors pick the right mode.
func (a *App) runOnText(command, text, language string) (string, bool) {
	args, err := curl.Split(command)
	if err != nil || len(args) == 0 {
		a.notify("Couldn't read the command "+command, toastError)
		return "", false
	}
	suffix := ""
	if language != "" {
		suffix = "." + language
	}
	file, err := os.CreateTemp("", "posting-*"+suffix)
	if err != nil {
		a.notify("Couldn't create a temporary file: "+err.Error(), toastError)
		return "", false
	}
	name := file.Name()
	defer os.Remove(name)
	_, err = file.WriteString(text)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		a.notify("Couldn't write a temporary file: "+err.Error(), toastError)
		return "", false
	}
	if err := a.runExternal(exec.Command(args[0], append(args[1:], name)...)); err != nil {
		a.notify("Couldn't run "+command+": "+err.Error(), toastError)
		return "", false
	}
	data, err := os.ReadFile(name)
	if err != nil {
		a.notify("Couldn't read the edited file: "+err.Error(), toastError)
		return "", false
	}
	return string(data), true
}

// openInEditor edits area's text in the configured editor and reports
// whether it changed. A read-only area is only shown there.
func (a *App) openInEditor(area *t.TextAreaState, language string) bool {
	if a.settings.Editor == "" {
		a.notify("No editor configured. Set $EDITOR or the editor setting.", toastWarning)
		return false
	}
	text, ok := a.runOnText(a.settings.Editor, area.GetText(), language)
	if !ok || area.ReadOnly.Peek() || text == area.GetText() {
		return false
	}
	area.SetText(text)
	return true
}

// openInPager shows area's text in the configured pager, or the JSON pager
// for JSON when one is set.
func (a *App) openInPager(area *t.TextAreaState, language string) {
	pager := a.settings.Pager
	if language == "json" && a.settings.PagerJSON != "" {
		pager = a.settings.PagerJSON
	}
	if pager == "" {
		a.notify("No pager configured. Set $PAGER or the pager setting.", toastWarning)
		return
	}
	a.runOnText(pager, area.GetText(), language)
}

// externalKeybinds open a body in the pager and editor, with Posting 2's
// keys. edited runs after the editor changes the text.
func (a *App) externalKeybinds(area *t.TextAreaState, language string, edited func()) []t.Keybind {
	return []t.Keybind{
		{Key: "f3", Name: "Pager", Action: func() { a.openInPager(area, language) }},
		{Key: "f4", Name: "Editor", Action: func() {
			if a.openInEditor(area, language) && edited != nil {
				edited()
			}
		}},
	}
}
