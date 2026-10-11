package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/model"
)

func TestEveryPaletteCommandRuns(tt *testing.T) {
	// Each top-level command either runs or opens a submenu; none may be
	// a placeholder without an action.
	app := testApp()
	for _, item := range app.paletteItems() {
		if item.Divider != "" {
			continue
		}
		if item.Action == nil && item.Children == nil {
			tt.Errorf("%q does nothing", item.Label)
		}
	}
}

func TestSidebarGradientPalette(tt *testing.T) {
	app := testApp()
	choose := func(label string) {
		tt.Helper()
		level := app.palette.CurrentLevel()
		for i, item := range level.Items {
			if item.Label == label {
				level.ListState.SelectIndex(i)
				pressOn(tt, app, "palette-input", "enter")
				return
			}
		}
		tt.Fatalf("palette item %q not found", label)
	}
	backgrounds := func() [2]string {
		buf := t.SerializeBuffer(t.RenderToBuffer(app, snapW, snapH), snapW, snapH)
		return [2]string{buf.Cells[0].Bg, buf.Cells[30*snapW].Bg}
	}
	original := backgrounds()
	if original[0] == original[1] {
		tt.Fatal("default sidebar is not a gradient")
	}
	app.openPalette()
	// The palette dims the app underneath it, so compare like-for-like.
	underPalette := backgrounds()
	choose("Sidebar gradient…")
	pressOn(tt, app, "palette-input", "down") // Preview Off.
	if got := backgrounds(); got[0] != got[1] || t.CurrentThemeName() != "galaxy" {
		tt.Fatalf("Off preview = %v, theme = %s", got, t.CurrentThemeName())
	}
	pressOn(tt, app, "palette-input", "escape") // Back restores the gradient.
	if got := backgrounds(); got != underPalette {
		tt.Fatalf("leaving the picker kept the preview: %v, want %v", got, underPalette)
	}
	choose("Sidebar gradient…")
	choose("lantern")
	pinned := backgrounds()
	if pinned == original || t.CurrentThemeName() != "galaxy" || app.palette.Visible.Peek() {
		tt.Fatal("choosing a gradient must change only the sidebar and close the palette")
	}
	app.openPalette()
	choose("Sidebar gradient…")
	pressOn(tt, app, "palette-input", "down")
	pressOn(tt, app, "palette-input", "escape")
	pressOn(tt, app, "palette-input", "escape")
	if got := backgrounds(); got != pinned {
		tt.Fatalf("cancelling lost the pinned gradient: %v, want %v", got, pinned)
	}
	app.openPalette()
	choose("Theme…")
	choose("aurora")
	app.openPalette()
	choose("Sidebar gradient…")
	selected := ""
	for _, item := range app.palette.CurrentLevel().Items {
		if item.Current {
			selected = item.Label
		}
	}
	if selected != "lantern" {
		tt.Fatalf("changing the app theme changed the selected gradient to %q", selected)
	}
	choose("Sidebar gradient off")
	if got := backgrounds(); got[0] != got[1] {
		tt.Fatalf("Off did not restore a flat background: %v", got)
	}
	app.openPalette()
	choose("Sidebar gradient…")
	choose("Follow app theme")
	following := backgrounds()
	app.openPalette()
	choose("Theme…")
	choose("galaxy")
	if got := backgrounds(); got == following || got != original {
		tt.Fatalf("Follow app theme did not track theme changes: %v", got)
	}
}

func TestExportYAMLRoundTrips(tt *testing.T) {
	app := testApp()
	req := sampleRequest(tt, "Create user")
	app.openRequest(req)
	app.exportYAML()
	if app.overlay.Peek() != "curl" || app.curlDialog.mode.Peek() != "yaml" {
		tt.Fatal("the YAML export dialog should be open")
	}
	back, err := collection.ParseRequest([]byte(app.curlDialog.text.GetText()), req.File)
	if err != nil {
		tt.Fatal(err)
	}
	want := app.current().Snapshot()
	if !reflect.DeepEqual(back, want) {
		tt.Fatalf("YAML export changed the request\n got %+v\nwant %+v", back, want)
	}
}

func TestDuplicateAndDeleteFromThePalette(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	app.openRequest(sampleRequest(tt, "Get user"))
	app.duplicateRequests([]model.Request{app.current().Snapshot()})
	if got := app.current().file.Peek(); got != "users/get-user-copy.posting.yaml" || !app.fileExists(got) {
		tt.Fatalf("duplicate = %q", got)
	}
	app.confirmDeleteRequests([]model.Request{app.current().Snapshot()})
	if app.overlay.Peek() != "confirm" {
		tt.Fatal("deleting should ask first")
	}
	app.confirm.Peek().onYes()
	if app.fileExists("users/get-user-copy.posting.yaml") || len(store.deleted) != 1 {
		tt.Fatalf("deleted = %v", store.deleted)
	}

	app.newTab()
	app.duplicateRequests([]model.Request{app.current().Snapshot()})
	if app.toast.Peek().kind != toastWarning {
		tt.Fatal("duplicating an unsaved request should explain why it can't")
	}
}

func TestLoadEnvironmentFileByPath(tt *testing.T) {
	app := New(Config{
		Collection:   model.SampleCollection(),
		Environments: flakySource{StaticEnvironments(model.SampleEnvironments())},
		UserHost:     "user@host",
	})
	app.openEnvFileDialog()
	app.envFile.path.SetText("missing.env")
	app.submitEnvFile()
	if app.envFile.err.Peek() == "" || app.overlay.Peek() != "envfile" {
		tt.Fatal("a file that can't be loaded should be reported in the dialog")
	}
}

func TestToggleSpacing(tt *testing.T) {
	app := testApp()
	app.toggleSpacing()
	if app.gap() != 0 {
		tt.Fatal("compact spacing should remove the gaps")
	}
	app.toggleSpacing()
	if app.gap() != 1 {
		tt.Fatal("standard spacing should restore them")
	}
}

func TestSnapshotLoadEnvironmentFileDialog(tt *testing.T) {
	app := testApp()
	app.openEnvFileDialog()
	t.AssertSnapshot(tt, app, snapW, snapH, "Load environment file dialog")
}

func TestBrowserOpenFailureIsShown(tt *testing.T) {
	app := testApp()
	app.openURL = func(string) error { return errors.New("no launcher") }
	app.openDocs()
	waitFor(tt, func() bool {
		toast := app.toast.Peek()
		return toast.kind == toastError && strings.Contains(toast.message, docsURL) && strings.Contains(toast.message, "no launcher")
	})
}

func TestLoadEnvironmentFileRemembersSuccessfulSwitch(tt *testing.T) {
	dir := tt.TempDir()
	staging, prod := filepath.Join(dir, "staging.env"), filepath.Join(dir, "prod.env")
	for file, content := range map[string]string{staging: "X=stage\n", prod: "X=prod\n"} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			tt.Fatal(err)
		}
	}
	memory := env.Memory{File: filepath.Join(tt.TempDir(), "environments.json")}
	app := New(Config{
		Collection:   model.SampleCollection(),
		Environments: env.Source{Dirs: []string{dir}},
		RememberEnvironment: func(files []string) {
			if err := memory.Remember(dir, files); err != nil {
				tt.Fatal(err)
			}
		},
		UserHost: "user@host",
	})
	app.switchEnvironment([]string{staging})
	app.openEnvFileDialog()
	app.envFile.path.SetText(prod)
	app.submitEnvFile()
	if app.envFile.err.Peek() != "" || app.variableValuesPeek()["X"] != "prod" {
		tt.Fatal("dialog did not load prod successfully")
	}
	if got := memory.Recall(dir); !reflect.DeepEqual(got, []string{prod}) {
		tt.Fatalf("remembered %v, want prod", got)
	}
	app.openEnvFileDialog()
	app.envFile.path.SetText(filepath.Join(dir, "missing.env"))
	app.submitEnvFile()
	if app.envFile.err.Peek() == "" || app.overlay.Peek() != "envfile" {
		tt.Fatal("failed load should keep the dialog open with an error")
	}
	if got := memory.Recall(dir); !reflect.DeepEqual(got, []string{prod}) {
		tt.Fatalf("failed load changed remembered environment to %v", got)
	}
}
