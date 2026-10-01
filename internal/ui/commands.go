package ui

import (
	"fmt"
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
	a.openLink("Documentation", docsURL)
}

// openLink opens url in the browser. Without a way to do that, it shows
// the address instead, headed by name. Opening waits for the system's
// launcher, so it happens in the background, and says if it fails.
func (a *App) openLink(name, url string) {
	open := a.openURL
	if open == nil {
		a.notify(name+": "+url, toastInfo)
		return
	}
	go func() {
		if err := open(url); err != nil {
			t.Dispatch(func() { a.notify("Couldn't open "+url+": "+err.Error(), toastError) })
		}
	}()
}

// duplicateRequests saves a copy of each of reqs beside it. A single copy
// opens in a tab. Several become the tree's selection instead of opening a
// tab each, so the next operation can act on them together.
func (a *App) duplicateRequests(reqs []model.Request) {
	switch len(reqs) {
	case 0:
		return
	case 1:
		if reqs[0].File == "" {
			a.notify("Save the request before duplicating it", toastWarning)
			return
		}
		if dup, ok := a.saveCopy(reqs[0]); ok {
			a.openSession(dup)
			a.notify("Duplicated as "+dup.File, toastSuccess)
		}
		return
	}
	copies := map[string]struct{}{}
	for _, req := range reqs {
		dup, ok := a.saveCopy(req)
		if !ok {
			break
		}
		copies[treeItem{Request: &dup}.key()] = struct{}{}
	}
	a.tree.Selection.Set(copies)
	if len(copies) == len(reqs) {
		a.notify(fmt.Sprintf("Duplicated %d requests", len(reqs)), toastSuccess)
	}
}

// saveCopy saves a copy of req beside it, under a file name no other
// request has. Like storeRequest, it reports whether the save worked; a
// failure has already been shown to the user.
func (a *App) saveCopy(req model.Request) (model.Request, bool) {
	dup := req.Clone()
	dup.Name = strings.TrimSpace(dup.Name + " (copy)")
	base := strings.TrimSuffix(dup.File, collection.FileSuffix)
	candidate := base + "-copy" + collection.FileSuffix
	for n := 2; a.fileExists(candidate); n++ {
		candidate = base + "-copy-" + strconv.Itoa(n) + collection.FileSuffix
	}
	dup.File = candidate
	return dup, a.storeRequest(dup)
}

// deleteListLimit is how many names the confirmation for deleting several
// requests lists before it counts the rest.
const deleteListLimit = 5

// confirmDeleteRequests asks before deleting the files of reqs: one request
// by name and file, several by count with their names listed.
func (a *App) confirmDeleteRequests(reqs []model.Request) {
	switch len(reqs) {
	case 0:
		return
	case 1:
		req := reqs[0]
		if req.File == "" {
			a.notify("This request hasn't been saved", toastWarning)
			return
		}
		a.askConfirm(confirmation{
			title:   "Delete request?",
			message: "[b]" + escapeMarkup(req.DisplayName()) + "[/] will be removed from the collection.\n[$TextMuted]" + escapeMarkup(req.File) + "[/]",
			confirm: "Delete",
			danger:  true,
			onYes:   func() { a.deleteRequests([]string{req.File}) },
		})
		return
	}
	files := make([]string, len(reqs))
	lines := []string{"These requests will be removed from the collection:"}
	for i, req := range reqs {
		files[i] = req.File
		if i < deleteListLimit {
			lines = append(lines, "  [b]"+escapeMarkup(req.DisplayName())+"[/]")
		}
	}
	if more := len(reqs) - deleteListLimit; more > 0 {
		lines = append(lines, fmt.Sprintf("  [$TextMuted]and %d more[/]", more))
	}
	a.askConfirm(confirmation{
		title:   fmt.Sprintf("Delete %d requests?", len(reqs)),
		message: strings.Join(lines, "\n"),
		confirm: "Delete",
		danger:  true,
		onYes:   func() { a.deleteRequests(files) },
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
	if a.env.remember != nil {
		a.env.remember(loaded.Files)
	}
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
