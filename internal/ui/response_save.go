package ui

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

type responseSaveForm struct {
	path *t.TextInputState
	err  t.Signal[string]
	// Capture the response when opening the dialog, so a new response in
	// the meantime cannot change which bytes the user saves.
	response *model.Response
}

func newResponseSaveForm() *responseSaveForm {
	return &responseSaveForm{path: t.NewTextInputState(""), err: t.NewSignal("")}
}

func (a *App) openResponseSave(resp *model.Response) {
	if resp == nil {
		return
	}
	name := "response"
	if u, err := url.Parse(resp.URL); err == nil {
		if base := path.Base(strings.ReplaceAll(u.Path, "\\", "/")); base != "." && base != ".." && base != "/" && base != "" {
			name = base
		}
	}
	if path.Ext(name) == "" {
		suffix := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg"}[resp.ContentType()]
		name += suffix
	}
	f := a.responseSave
	f.response = resp
	f.path.SetText(name)
	f.path.SelectAll()
	f.err.Set("")
	a.overlay.Set("response-save")
	t.RequestFocus("response-save-path")
}

func (a *App) submitResponseSave() {
	f := a.responseSave
	name := strings.TrimSpace(f.path.GetText())
	if name == "" {
		f.err.Set("Enter a file path")
		return
	}
	if name == "~" || strings.HasPrefix(name, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			name = filepath.Join(home, name[1:])
		}
	}
	// Never silently overwrite a user's file, including a symlink.
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			f.err.Set("A file already exists at this path. Choose another name.")
		} else {
			f.err.Set(err.Error())
		}
		return
	}
	_, err = file.Write(f.response.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		f.err.Set(err.Error())
		return
	}
	a.closeOverlay()
	a.notify("Saved "+name, toastSuccess)
}

type responseSaveOverlay struct {
	app     *App
	visible bool
}

func (o responseSaveOverlay) Build(ctx t.BuildContext) t.Widget {
	a, theme := o.app, ctx.Theme()
	f := a.responseSave
	message := t.Text{Content: "Save the original response body. Relative paths start from the directory Posting was started in. Existing files are not overwritten.", Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}}
	if err := f.err.Get(); err != "" {
		message.Content = err
		message.Style.ForegroundColor = theme.ErrorText
	}
	return modal{
		Visible: o.visible, Title: "Save response body", Width: t.Cells(72), OnDismiss: a.closeOverlay,
		Child: t.Column{
			Style: t.Style{Width: t.Flex(1)}, Spacing: 1,
			Children: []t.Widget{
				input{ID: "response-save-path", State: f.path, Placeholder: "File path", OnSubmit: func(string) { a.submitResponseSave() }},
				message,
				t.Row{Style: t.Style{Width: t.Flex(1)}, Spacing: 2, MainAlign: t.MainAxisEnd, Children: []t.Widget{
					t.Button{ID: "response-save-cancel", Label: "Cancel", OnPress: a.closeOverlay, Click: func(t.MouseEvent) { a.closeOverlay() }},
					t.Button{ID: "response-save-submit", Label: "Save", Variant: t.ButtonSuccess, OnPress: a.submitResponseSave, Click: func(t.MouseEvent) { a.submitResponseSave() }},
				}},
			},
		},
	}
}
