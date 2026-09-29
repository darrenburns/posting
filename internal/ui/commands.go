package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

// docsURL is where the "Open documentation" command goes.
const docsURL = "https://posting.sh"

// exportYAML shows the request as a Posting request file and copies it.
func (a *App) exportYAML() {
	s := a.current()
	if s == nil {
		return
	}
	data, err := collection.MarshalRequest(s.Snapshot())
	if err != nil {
		a.notify("Couldn't export: "+err.Error(), toastError)
		return
	}
	f := a.curlDialog
	f.mode.Set("yaml")
	f.err.Set("")
	f.text.SetText(string(data))
	f.text.CursorIndex.Set(0)
	a.overlay.Set("curl")
	t.RequestFocus("curl-text")
	t.SetClipboard(t.SystemClipboard, string(data))
	a.notify("Copied request YAML to clipboard", toastSuccess)
}

// toggleSpacing switches between standard and compact spacing for the
// session.
func (a *App) toggleSpacing() {
	if a.spacing.Peek() == "compact" {
		a.spacing.Set("standard")
	} else {
		a.spacing.Set("compact")
	}
}

func (a *App) openDocs() {
	if a.openURL == nil {
		a.notify("Documentation: "+docsURL, toastInfo)
		return
	}
	if err := a.openURL(docsURL); err != nil {
		a.notify("Couldn't open "+docsURL+": "+err.Error(), toastError)
	}
}

// duplicateRequest saves a copy of req beside it and opens the copy.
func (a *App) duplicateRequest(req model.Request) {
	if req.File == "" {
		a.notify("Save the request before duplicating it", toastWarning)
		return
	}
	dup := req.Clone()
	dup.Name = strings.TrimSpace(dup.Name + " (copy)")
	base := strings.TrimSuffix(dup.File, collection.FileSuffix)
	candidate := base + "-copy" + collection.FileSuffix
	for n := 2; a.fileExists(candidate); n++ {
		candidate = base + "-copy-" + strconv.Itoa(n) + collection.FileSuffix
	}
	dup.File = candidate
	if !a.storeRequest(dup) {
		return
	}
	a.openSession(dup)
	a.notify("Duplicated as "+dup.File, toastSuccess)
}

// confirmDelete asks before deleting req's file.
func (a *App) confirmDelete(req model.Request) {
	if req.File == "" {
		a.notify("This request hasn't been saved", toastWarning)
		return
	}
	file := req.File
	a.askConfirm(confirmation{
		title:   "Delete request?",
		message: "[b]" + escapeMarkup(req.DisplayName()) + "[/] will be removed from the collection.\n[$TextMuted]" + escapeMarkup(file) + "[/]",
		confirm: "Delete",
		danger:  true,
		onYes:   func() { a.deleteRequest(file) },
	})
}

// reloadCollection rereads the collection from disk now.
func (a *App) reloadCollection() {
	if a.watcher == nil {
		a.notify("This collection isn't stored on disk", toastInfo)
		return
	}
	root, problems := a.watcher.Load()
	a.replaceCollection(root)
	if len(problems) > 0 {
		a.notify("Reloaded, but couldn't load "+problems[0].Error(), toastError)
		return
	}
	a.notify("Reloaded the collection", toastSuccess)
}

func (a *App) copyResponseBody() {
	s := a.current()
	if s == nil || s.response.Peek() == nil {
		a.notify("There's no response to copy", toastWarning)
		return
	}
	t.SetClipboard(t.SystemClipboard, s.responseBody.GetText())
	a.notify("Copied response body", toastSuccess)
}

// ---------------------------------------------------------------------------
// Loading an environment file by path

type envFileForm struct {
	path *t.TextInputState
	err  t.Signal[string]
}

func newEnvFileForm() *envFileForm {
	return &envFileForm{path: t.NewTextInputState(""), err: t.NewSignal("")}
}

func (a *App) openEnvFileDialog() {
	a.envFile.path.SetText("")
	a.envFile.err.Set("")
	a.overlay.Set("envfile")
	t.RequestFocus("envfile-path")
}

// submitEnvFile loads the typed files (separated by commas to layer them)
// as the environment.
func (a *App) submitEnvFile() {
	var files []string
	for _, part := range strings.Split(a.envFile.path.GetText(), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "~" || strings.HasPrefix(part, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				part = filepath.Join(home, part[1:])
			}
		}
		if abs, err := filepath.Abs(part); err == nil {
			part = abs
		}
		files = append(files, part)
	}
	if len(files) == 0 {
		a.envFile.err.Set("Enter the path of a .env file")
		return
	}
	loaded, err := a.env.source.Load(files)
	if err != nil {
		a.envFile.err.Set(err.Error())
		return
	}
	a.closeOverlay()
	a.setEnvironment(loaded)
	a.notify("Switched to "+loaded.Name, toastInfo)
	t.RequestFocus(urlInputID)
}

type envFileOverlay struct {
	app     *App
	visible bool
}

func (o envFileOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := o.app
	f := a.envFile
	message := t.Text{Content: "Relative paths start from the directory Posting was started in. Separate several files with commas to layer them.", Wrap: t.WrapSoft, Style: t.Style{ForegroundColor: theme.TextMuted, Width: t.Flex(1)}}
	if err := f.err.Get(); err != "" {
		message = t.Text{Content: err, Wrap: t.WrapSoft, Style: t.Style{ForegroundColor: theme.ErrorText, Width: t.Flex(1)}}
	}
	return modal{
		Visible:   o.visible,
		Title:     "Load environment file",
		Width:     t.Cells(72),
		OnDismiss: a.closeOverlay,
		Child: t.Column{
			Style:   t.Style{Width: t.Flex(1)},
			Spacing: 1,
			Children: []t.Widget{
				input{ID: "envfile-path", State: f.path, Placeholder: "e.g. envs/staging.env", OnSubmit: func(string) { a.submitEnvFile() }},
				message,
				t.Row{Spacing: 2, MainAlign: t.MainAxisEnd, Style: t.Style{Width: t.Flex(1), Height: t.Cells(1)}, Children: []t.Widget{
					t.Button{ID: "envfile-cancel", Label: "Cancel", OnPress: a.closeOverlay, Click: func(t.MouseEvent) { a.closeOverlay() }},
					t.Button{ID: "envfile-load", Label: "Load", Variant: t.ButtonPrimary, OnPress: a.submitEnvFile, Click: func(t.MouseEvent) { a.submitEnvFile() }},
				}},
			},
		},
	}
}
