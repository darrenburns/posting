package ui

// Captures the screens shown on the docs homepage. `make docs-screens` runs
// this with HOMEPAGE_OUT set, then docs/scripts/home_screens.py turns the
// SVG captures into theme-switchable partials. Keep the theme list in step
// with PALETTES there.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// Match the populated workspace in docs/assets/themes, using the current UI.
func homeShowcase(tt *testing.T, theme string) *App {
	tt.Helper()
	app := homeApp(theme)
	collection := model.SampleCollection()
	collection.Name = "atlas-api"
	type request struct {
		name, path string
		method     model.Method
	}
	for _, group := range []struct {
		name     string
		requests []request
	}{
		{"auth", []request{
			{"Refresh session", "/auth/refresh", model.MethodPost},
			{"Revoke session", "/auth/session", model.MethodDelete},
		}},
		{"projects", []request{
			{"List projects", "/projects", model.MethodGet},
			{"Get project", "/projects/12", model.MethodGet},
			{"Create project", "/projects", model.MethodPost},
			{"Update project", "/projects/12", model.MethodPatch},
			{"Archive project", "/projects/12", model.MethodDelete},
		}},
		{"teams", []request{
			{"List teams", "/teams", model.MethodGet},
			{"Team members", "/teams/7/members", model.MethodGet},
			{"Invite member", "/teams/7/members", model.MethodPost},
			{"Remove member", "/teams/7/members/42", model.MethodDelete},
		}},
		{"webhooks", []request{
			{"List webhooks", "/webhooks", model.MethodGet},
			{"Create webhook", "/webhooks", model.MethodPost},
			{"Test delivery", "/webhooks/3/test", model.MethodPost},
			{"Delete webhook", "/webhooks/3", model.MethodDelete},
		}},
	} {
		var folder *model.Collection
		for _, child := range collection.Children {
			if child.Name == group.name {
				folder = child
			}
		}
		if folder == nil {
			folder = &model.Collection{Name: group.name, Path: group.name}
			collection.Children = append(collection.Children, folder)
		}
		for _, spec := range group.requests {
			req := model.NewRequest()
			req.Name, req.Method, req.URL = spec.name, spec.method, "${BASE_URL}"+spec.path
			req.File = filepath.Join(group.name, strings.ReplaceAll(strings.ToLower(spec.name), " ", "-")+".posting.yaml")
			folder.Requests = append(folder.Requests, req)
		}
	}
	collection.Sort()
	app.replaceCollection(collection)
	app.sidebarSplit.SetPosition(0.27)

	get := sampleRequest(tt, "List users")
	get.Headers = []model.KeyValue{
		{Name: "Accept", Value: "application/json", Enabled: true},
		{Name: "Authorization", Value: "Bearer ${API_TOKEN}", Enabled: true},
		{Name: "User-Agent", Value: "Posting/3.0", Enabled: true},
		{Name: "Cache-Control", Value: "no-cache", Enabled: true},
		{Name: "X-Request-ID", Value: "${$uuid}", Enabled: true},
		{Name: "X-Client-Version", Value: "2026.10", Enabled: true},
		{Name: "Origin", Value: "https://app.example.com", Enabled: true},
	}
	app.openRequest(get)
	app.keepSession(app.active.Peek())
	app.openRequests([]model.Request{sampleRequest(tt, "Create user"), sampleRequest(tt, "Delete user")})
	app.openRequest(get)
	moveTreeCursor(tt, app, "List users")

	response := fixedResponse()
	response.Body = []byte(`{"users":[{"id":42,"name":"Ada Lovelace","email":"ada@example.com","role":"admin","active":true},{"id":43,"name":"Grace Hopper","email":"grace@example.com","role":"editor","active":true},{"id":44,"name":"Alan Turing","email":"alan@example.com","role":"viewer","active":false}],"page":1,"per_page":20,"total":128,"next":"/users?page=2"}`)
	s := app.current()
	s.showResponse(response, nil)
	s.phase.Set(exchangeDone)
	return app
}

func TestGenerateHomepageScenes(tt *testing.T) {
	out := os.Getenv("HOMEPAGE_OUT")
	if out == "" {
		tt.Skip("HOMEPAGE_OUT not set")
	}
	const width, height = 120, 36
	themes := []string{"galaxy", "aurora", "lantern", "midnight-ember", "kintsugi", "neon-reef", "cyberdeck", "catppuccin-latte"}
	scenes := map[string]func(string) *App{
		"response": func(th string) *App {
			return homeShowcase(tt, th)
		},
		"jump": func(th string) *App {
			app := homeShowcase(tt, th)
			app.jump.Activate()
			return app
		},
		"palette": func(th string) *App {
			app := homeShowcase(tt, th)
			app.openPalette()
			return app
		},
		"themes": func(th string) *App {
			app := homeShowcase(tt, th)
			app.palette.SetItems(app.paletteItems())
			app.palette.Open()
			app.palette.PushLevel(themesTitle, app.themeItems())
			return app
		},
		"variables": func(th string) *App {
			app := homeShowcase(tt, th)
			app.openVariables()
			return app
		},
		"curlhint": func(th string) *App {
			app := homeShowcase(tt, th)
			app.current().url.SetText("curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\":\"Ada\"}'")
			return app
		},
		"curlexport": func(th string) *App {
			app := homeShowcase(tt, th)
			app.openRequest(sampleRequest(tt, "Create user"))
			app.copyExport()
			return app
		},
		"history": func(th string) *App {
			app := homeShowcase(tt, th)
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
	for name, build := range scenes {
		for _, th := range themes {
			tt.Run(name+"/"+th, func(tt *testing.T) {
				app := build(th)
				p := t.NewPilot(tt, app, width, height)
				if name == "response" {
					row := app.current().headers.rowAt(7)
					fieldID := app.current().headers.inputID(row.id, "key")
					p.Click(fieldID)
					p.Type("Accept-")
					if p.FocusedID() != fieldID || !row.keySuggestions.IsVisible() {
						tt.Fatalf("header autocomplete is not focused and visible:\n%s", p.ScreenText())
					}
				}
				opts := t.DefaultSVGOptions()
				opts.FontFamily = "JetBrains Mono, monospace"
				opts.FontSize, opts.CellWidth, opts.Padding = 18, 10.8, 0
				svg := t.BufferToSVG(p.Buffer(), width, height, opts)
				if err := os.WriteFile(filepath.Join(out, name+"-"+th+".svg"), []byte(svg), 0o644); err != nil {
					tt.Fatal(err)
				}
			})
		}
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
	data, _ := json.MarshalIndent(allThemes, "", " ")
	if err := os.WriteFile(filepath.Join(out, "themes.json"), data, 0o644); err != nil {
		tt.Fatal(err)
	}
}
