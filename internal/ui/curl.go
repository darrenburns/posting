package ui

import (
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
)

// curlForm is the state of the curl import and export dialogs.
type curlForm struct {
	// mode is "import" or "export".
	mode    t.Signal[string]
	text    *t.TextAreaState
	scroll  *t.ScrollState
	err     t.Signal[string]
	resolve t.Signal[bool] // export: substitute variables
}

func newCurlForm() *curlForm {
	return &curlForm{
		mode:    t.NewSignal("import"),
		text:    t.NewTextAreaState(""),
		scroll:  t.NewScrollState(),
		err:     t.NewSignal(""),
		resolve: t.NewSignal(true),
	}
}

// importCurl replaces the current tab's request with the one a curl command
// describes. The tab keeps its name and file, so pasting over a saved
// request updates that request.
func (a *App) importCurl(command string) bool {
	s := a.current()
	if s == nil {
		return false
	}
	req, err := curl.Parse(command)
	if err != nil {
		a.notify("Couldn't import curl command: "+err.Error(), toastError)
		return false
	}
	current := s.Snapshot()
	req.Name, req.Description, req.File, req.Scripts = current.Name, current.Description, current.File, current.Scripts
	s.Load(req)
	s.dirty.Set(true)
	a.notify("Imported curl command", toastSuccess)
	return true
}

// submitURL sends the request, or imports the URL bar's contents when a curl
// command has been pasted or typed there. A command ending in a backslash
// continues on the next line, as in a shell, so a multi-line command pasted
// line by line is imported once it is complete.
func (a *App) submitURL(text string) {
	s := a.current()
	if s == nil || !curl.IsCommand(text) {
		a.send()
		return
	}
	if strings.HasSuffix(strings.TrimRight(text, " "), "\\") {
		s.url.SetText(strings.TrimSuffix(strings.TrimRight(text, " "), "\\") + " ")
		s.url.CursorEnd()
		return
	}
	if a.importCurl(text) {
		t.RequestFocus(urlInputID)
	}
}

// curlCommand is the current request as a curl command.
func (a *App) curlCommand(resolve bool) (string, error) {
	s := a.current()
	if s == nil {
		return "", nil
	}
	req := s.Snapshot()
	if resolve {
		resolved, err := model.Resolve(req, model.MapLookup(a.variableValuesPeek()))
		if err != nil {
			return "", err
		}
		req = resolved
	}
	return curl.Format(req, curl.FormatOptions{ExtraArgs: a.settings.CurlExportExtraArgs, Multiline: true}), nil
}

// copyAsCurl copies the request as a curl command and shows it.
func (a *App) copyAsCurl() {
	a.openCurlExport()
	if text := a.curlDialog.text.GetText(); text != "" {
		t.SetClipboard(t.SystemClipboard, text)
		a.notify("Copied curl command to clipboard", toastSuccess)
	}
}

func (a *App) openCurlExport() {
	f := a.curlDialog
	f.mode.Set("export")
	a.refreshCurlExport()
	a.overlay.Set("curl")
	t.RequestFocus("curl-text")
}

func (a *App) refreshCurlExport() {
	f := a.curlDialog
	command, err := a.curlCommand(f.resolve.Peek())
	f.err.Set("")
	if err != nil {
		f.err.Set(err.Error() + " — showing the command with variables left in")
		command, _ = a.curlCommand(false)
	}
	f.text.SetText(command)
	f.text.CursorIndex.Set(0)
}

func (a *App) openCurlImport() {
	f := a.curlDialog
	f.mode.Set("import")
	f.text.SetText("")
	f.err.Set("")
	a.overlay.Set("curl")
	t.RequestFocus("curl-text")
}

func (a *App) submitCurlImport() {
	f := a.curlDialog
	if strings.TrimSpace(f.text.GetText()) == "" {
		f.err.Set("Paste a curl command first")
		return
	}
	if _, err := curl.Parse(f.text.GetText()); err != nil {
		f.err.Set(err.Error())
		return
	}
	a.closeOverlay()
	a.importCurl(f.text.GetText())
	t.RequestFocus(urlInputID)
}

type curlOverlay struct {
	app     *App
	visible bool
}

func (o curlOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := o.app
	f := a.curlDialog
	importing := f.mode.Get() == "import"
	title := "Export as curl"
	hint := "The command has been copied to the clipboard. Select text with your terminal to copy part of it."
	buttons := []t.Widget{
		t.Button{ID: "curl-close", Label: "Close", OnPress: a.closeOverlay, Click: func(t.MouseEvent) { a.closeOverlay() }},
		t.Button{ID: "curl-copy", Label: "Copy", Variant: t.ButtonPrimary, OnPress: a.copyCurlText, Click: func(t.MouseEvent) { a.copyCurlText() }},
	}
	var toggle t.Widget = t.EmptyWidget{}
	if importing {
		title = "Import curl command"
		hint = "Paste a curl command, for example from your browser's \"Copy as cURL\". It replaces the request in the current tab."
		buttons = []t.Widget{
			t.Button{ID: "curl-close", Label: "Cancel", OnPress: a.closeOverlay, Click: func(t.MouseEvent) { a.closeOverlay() }},
			t.Button{ID: "curl-import", Label: "Import", Variant: t.ButtonSuccess, OnPress: a.submitCurlImport, Click: func(t.MouseEvent) { a.submitCurlImport() }},
		}
	} else {
		toggle = segmented{
			ID:       "curl-resolve",
			Options:  []choice{{Value: "resolved", Label: "Values"}, {Value: "variables", Label: "Variables"}},
			Selected: map[bool]string{true: "resolved", false: "variables"}[f.resolve.Get()],
			OnChange: func(value string) {
				f.resolve.Set(value == "resolved")
				a.refreshCurlExport()
			},
		}
	}
	message := t.Text{Content: hint, Wrap: t.WrapSoft, Style: t.Style{ForegroundColor: theme.TextMuted, Width: t.Flex(1)}}
	if err := f.err.Get(); err != "" {
		message = t.Text{Content: err, Wrap: t.WrapSoft, Style: t.Style{ForegroundColor: theme.ErrorText, Width: t.Flex(1)}}
	}
	area := t.TextArea{
		ID:          "curl-text",
		State:       f.text,
		ScrollState: f.scroll,
		Placeholder: "curl https://example.com -H 'Accept: application/json'",
		Highlighter: a.bodyHighlighter(theme, "bash", false),
		Style:       t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
	}
	f.text.ReadOnly.Set(!importing)
	return modal{
		Visible:   o.visible,
		Title:     title,
		Width:     t.Cells(96),
		Height:    t.Cells(24),
		OnDismiss: a.closeOverlay,
		Child: t.Column{
			Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Spacing: 1,
			Children: []t.Widget{
				curlKeys{app: a, importing: importing, child: scrollingArea("curl-text", f.scroll, theme.Surface, area)},
				message,
				t.Row{Spacing: 2, Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: append([]t.Widget{toggle, t.Spacer{}}, buttons...)},
			},
		},
	}
}

func (a *App) copyCurlText() {
	t.SetClipboard(t.SystemClipboard, a.curlDialog.text.GetText())
	a.notify("Copied curl command to clipboard", toastSuccess)
}

// curlKeys lets ctrl+j (the send key) import from inside the text area,
// where enter adds a new line.
type curlKeys struct {
	fillParent
	app       *App
	importing bool
	child     t.Widget
}

func (k curlKeys) Keybinds() []t.Keybind {
	if k.importing {
		return []t.Keybind{
			{Key: "ctrl+j", Name: "Import", Action: k.app.submitCurlImport},
			{Key: "alt+enter", Name: "Import", Action: k.app.submitCurlImport, Hidden: true},
		}
	}
	return []t.Keybind{{Key: "y", Name: "Copy", Action: k.app.copyCurlText}}
}

func (k curlKeys) Build(t.BuildContext) t.Widget {
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: []t.Widget{k.child}}
}
