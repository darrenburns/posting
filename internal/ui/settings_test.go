package ui

import (
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/config"
	"github.com/darrenburns/posting/internal/model"
)

func settingsApp(tt *testing.T, change func(*config.Settings)) *App {
	tt.Helper()
	settings := config.Defaults()
	change(&settings)
	return New(Config{
		Version:      "3.0.0-dev",
		Collection:   model.SampleCollection(),
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
		Settings:     &settings,
	})
}

func TestSettingsShapeTheLayout(tt *testing.T) {
	app := settingsApp(tt, func(s *config.Settings) {
		s.Layout = "horizontal"
		s.CollectionBrowser.Position = "right"
		s.Heading.ShowHost = false
		s.Heading.ShowVersion = false
		s.Response.ShowSizeAndTime = false
		s.Spacing = "compact"
	})
	app.openRequest(sampleRequest(tt, "List users"))
	app.current().showResponse(fixedResponse(), nil)
	app.current().phase.Set(exchangeDone)
	t.AssertSnapshot(tt, app, snapW, snapH, "Configured: side by side, collection on the right, compact spacing, no host, version or response size")
}

func TestSettingsHideTheCollectionAndHeading(tt *testing.T) {
	app := settingsApp(tt, func(s *config.Settings) {
		s.CollectionBrowser.ShowOnStartup = false
		s.Heading.Visible = false
	})
	t.AssertSnapshot(tt, app, snapW, snapH, "Configured: no collection browser and no heading row")
}

func TestKeymapRebindsActions(tt *testing.T) {
	app := settingsApp(tt, func(s *config.Settings) {
		s.Keymap = map[string]string{"send-request": "ctrl+r", "commands": "ctrl+k,f2"}
	})
	keys := map[string]string{}
	for _, bind := range app.Keybinds() {
		keys[bind.Key] = bind.Name
	}
	if keys["ctrl+r"] != "Send" || keys["ctrl+k"] != "Commands" || keys["f2"] != "Commands" {
		tt.Fatalf("keymap not applied: %v", keys)
	}
	if _, ok := keys["ctrl+j"]; ok {
		tt.Fatal("rebinding send should remove its default key")
	}
	if app.keyHint("send-request") != "ctrl+r" {
		tt.Fatalf("hint = %q", app.keyHint("send-request"))
	}
}

func TestPrettifyJSONSetting(tt *testing.T) {
	app := settingsApp(tt, func(s *config.Settings) { s.Response.PrettifyJSON = false })
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	if got := s.responseBody.GetText(); got != string(fixedResponse().Body) {
		tt.Fatalf("body was reformatted: %q", got)
	}
}

func TestFocusOnRequestOpen(tt *testing.T) {
	app := settingsApp(tt, func(s *config.Settings) { s.Focus.OnRequestOpen = "body" })
	app.openRequest(sampleRequest(tt, "Create user"))
	if got := app.current().requestTab.Peek(); got != "body" {
		tt.Fatalf("request tab = %q, want body", got)
	}
}
