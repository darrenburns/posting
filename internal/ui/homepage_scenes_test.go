package ui

// Captures the screens shown on the docs homepage. `make docs-screens` runs
// this with HOMEPAGE_OUT set, then docs/scripts/home_screens.py turns the
// captures into HTML. Keep the theme list in step with PALETTES there.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/config"
	"github.com/darrenburns/posting/v3/internal/model"
)

func homeApp(theme string) *App {
	s := config.Defaults()
	s.Theme = theme
	return New(Config{
		Version:      "3.0.0",
		Collection:   model.SampleCollection(),
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "you@devbox",
		Settings:     &s,
	})
}

func TestGenerateHomepageScenes(tt *testing.T) {
	out := os.Getenv("HOMEPAGE_OUT")
	if out == "" {
		tt.Skip("HOMEPAGE_OUT not set")
	}
	themes := []string{"galaxy", "aurora", "lantern", "midnight-ember", "kintsugi", "neon-reef", "cyberdeck", "catppuccin-latte"}
	scenes := map[string]func(string) *App{
		"response": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "List users"))
			s := app.current()
			s.showResponse(fixedResponse(), nil)
			s.phase.Set(exchangeDone)
			return app
		},
		"jump": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "List users"))
			s := app.current()
			s.showResponse(fixedResponse(), nil)
			s.phase.Set(exchangeDone)
			app.jump.Activate()
			return app
		},
		"palette": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "List users"))
			s := app.current()
			s.showResponse(fixedResponse(), nil)
			s.phase.Set(exchangeDone)
			app.openPalette()
			return app
		},
		"themes": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "List users"))
			s := app.current()
			s.showResponse(fixedResponse(), nil)
			s.phase.Set(exchangeDone)
			app.palette.SetItems(app.paletteItems())
			app.palette.Open()
			app.palette.PushLevel(themesTitle, app.themeItems())
			return app
		},
		"variables": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "List users"))
			app.openVariables()
			return app
		},
		"curlhint": func(th string) *App {
			app := homeApp(th)
			app.current().url.SetText("curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\":\"Ada\"}'")
			return app
		},
		"curlexport": func(th string) *App {
			app := homeApp(th)
			app.openRequest(sampleRequest(tt, "Create user"))
			app.copyExport()
			return app
		},
		"history": func(th string) *App {
			app := homeApp(th)
			app.layout.Set(layoutHorizontal)
			app.sidebarTab.Set("history")
			sent := time.Date(2026, 9, 28, 14, 30, 0, 0, time.UTC)
			var history []model.HistoryEntry
			for i, name := range []string{"List users", "Create user", "Get user"} {
				req := sampleRequest(tt, name)
				resp := fixedResponse()
				resp.Method = req.Method
				if i == 2 {
					resp.StatusCode, resp.Reason = 404, "Not Found"
				}
				history = append(history, sentEntry(int64(i+1), req, resp, sent.Add(-time.Duration(i)*time.Minute)))
			}
			app.history.Set(history)
			app.historyList.SetItems(history)
			app.openHistory(history[0])
			app.current().responseTab.Set("trace")
			return app
		},
	}
	result := map[string]map[string]t.SerializedBuffer{}
	for name, build := range scenes {
		result[name] = map[string]t.SerializedBuffer{}
		for _, th := range themes {
			app := build(th)
			buf := t.RenderToBuffer(app, snapW, snapH)
			result[name][th] = t.SerializeBuffer(buf, snapW, snapH)
		}
	}
	data, _ := json.Marshal(result)
	if err := os.WriteFile(filepath.Join(out, "scenes.json"), data, 0o644); err != nil {
		tt.Fatal(err)
	}
	allThemes := map[string]any{}
	for _, name := range t.ThemeNames() {
		th, _ := t.GetTheme(name)
		fields := map[string]any{}
		v := reflect.ValueOf(th)
		for i := 0; i < v.NumField(); i++ {
			if c, ok := v.Field(i).Interface().(t.Color); ok {
				fields[v.Type().Field(i).Name] = c.Hex()
			} else {
				fields[v.Type().Field(i).Name] = v.Field(i).Interface()
			}
		}
		allThemes[name] = fields
	}
	data, _ = json.MarshalIndent(allThemes, "", " ")
	if err := os.WriteFile(filepath.Join(out, "themes.json"), data, 0o644); err != nil {
		tt.Fatal(err)
	}
}
