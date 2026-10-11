package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/rivo/uniseg"

	"github.com/darrenburns/posting/internal/model"
)

func TestVariableChoices(tt *testing.T) {
	app := testApp()
	app.sessionVars.Set(map[string]string{"SESSION_ONLY": "x"})
	choices := app.variableChoices()
	got := map[string]string{}
	for _, s := range choices.list {
		got[s.Label] = s.Description
		if s.Value != s.Label {
			tt.Errorf("%s inserts %q", s.Label, s.Value)
		}
	}
	if got["${BASE_URL}"] != "http://localhost:8000" || got["${SESSION_ONLY}"] != "x" {
		tt.Fatalf("choices = %v", got)
	}
	if got["${API_TOKEN}"] != "••••••••" || got["${PASSWORD}"] != "••••••••" {
		tt.Fatalf("secrets must be masked: %v", got)
	}

	before := choices.key
	app.switchEnvironment([]string{"staging.env"})
	if app.variableChoices().key == before {
		tt.Fatal("switching environment should change the choices")
	}
}

func TestDollarCompletesVariablesMidURL(tt *testing.T) {
	app := testApp()
	s := app.current()
	s.url.SetText("https://api.test/users/")
	s.url.CursorEnd()
	pasteInto(tt, app, urlInputID, "$")
	if !s.urlVars.state.Visible.Peek() {
		tt.Fatal("typing $ after a / should offer the variables")
	}
}

func urlCompletionApp() *App {
	app := testApp()
	root := &model.Collection{Name: "Completion", Children: []*model.Collection{{Name: "Nested"}}}
	for _, url := range []string{
		"http://localhost:3000/users",
		"http://localhost:3000/users/:id/posts",
		"http://localhost:3000/users/:id/settings",
		"http://localhost:3000/teams/members",
		"http://localhost:3000/teams/members?limit=10",
		"https://localhost:3000/secure",
		"http://localhost:4000/other",
		"${BASE_URL}/users/:id/posts",
		"http://localhost:3000/cafe\u0301/日本語",
		"http://localhost:3000/a%2Fb/details",
		"http://localhost:3000/users//double",
	} {
		req := model.NewRequest()
		req.Name, req.File, req.URL = "Request", url+".posting.yaml", url
		root.Children[0].Requests = append(root.Children[0].Requests, req)
	}
	app.replaceCollection(root)
	return app
}

func pasteURL(tt *testing.T, screen *screen, text string) {
	tt.Helper()
	if !screen.focus.Focused().(t.PasteHandler).HandlePaste(text) {
		tt.Fatal("URL input rejected paste")
	}
	screen.render()
}

func TestURLCompletionTraversal(tt *testing.T) {
	app := urlCompletionApp()
	s := app.current()
	screen := sidebarScreen(tt, app, urlInputID)
	for _, step := range []struct{ typeText, key, want string }{
		{"http://local", "tab", "http://localhost:3000"},
		{"/us", "enter", "http://localhost:3000/users"},
		{"/", "tab", "http://localhost:3000/users/:id"},
		{"/", "down", "http://localhost:3000/users/:id/"},
		{"", "enter", "http://localhost:3000/users/:id/settings"},
	} {
		if step.typeText != "" {
			pasteURL(tt, screen, step.typeText)
		}
		// The live focus manager checks capture before it checks Tab bindings.
		// Calling the binding alone would miss a regression to focus traversal.
		if step.key == "tab" && !screen.focus.Focused().(t.KeyCapturer).CapturesKey("tab") {
			tt.Fatal("Tab would move focus instead of accepting the completion")
		}
		screen.pressKey(tt, step.key)
		if got := s.url.GetText(); got != step.want {
			tt.Fatalf("after %q and %s: URL = %q, want %q", step.typeText, step.key, got, step.want)
		}
		if s.phase.Peek() != exchangeIdle {
			tt.Fatal("completion sent the request")
		}
	}
	if !s.dirty.Peek() {
		tt.Fatal("completing a URL must mark the request as edited")
	}
	if screen.focus.Focused().(t.KeyCapturer).CapturesKey("tab") {
		tt.Fatal("Tab must navigate normally after accepting a completion")
	}
}

func TestURLCompletionBoundaries(tt *testing.T) {
	for _, tc := range []struct {
		name, before, after, want string
	}{
		{"host without scheme", "localhost:3", "", "http://localhost:3000"},
		{"path without scheme", "localhost:3000/users/:id/p", "", "localhost:3000/users/:id/posts"},
		{"explicit scheme", "https://local", "", "https://localhost:3000"},
		{"explicit scheme isolates paths", "https://localhost:3000/us", "", ""},
		{"host casing", "HTTP://LOCALHOST:3000/te", "", "HTTP://LOCALHOST:3000/teams"},
		{"different port", "http://localhost:4000/", "", "http://localhost:4000/other"},
		{"own branch only", "http://localhost:3000/teams/", "", "http://localhost:3000/teams/members"},
		{"replace whole segment", "http://localhost:3000/us", "eless/:id/posts?q=1#top", "http://localhost:3000/users/:id/posts?q=1#top"},
		{"replace base", "local", "wrong/users?q=1", "http://localhost:3000/users?q=1"},
		{"literal template", "${BASE_URL}/us", "", "${BASE_URL}/users"},
		{"variable priority", "http://localhost:3000/$BASE", "", "http://localhost:3000/${BASE_URL}"},
		{"variable suffix", "${BAS", "E_URL}/users", "${BASE_URL}/users"},
		{"query variable", "http://localhost:3000/users?q=$BASE", "", "http://localhost:3000/users?q=${BASE_URL}"},
		{"grapheme cursor", "http://localhost:3000/cafe\u0301/日", "wrong?x=1", "http://localhost:3000/cafe\u0301/日本語?x=1"},
		{"encoded slash", "http://localhost:3000/a%2Fb/", "", "http://localhost:3000/a%2Fb/details"},
		{"empty segment", "http://localhost:3000/users//", "", "http://localhost:3000/users//double"},
		{"no fuzzy path", "http://localhost:3000/ues", "", ""},
		{"case sensitive path", "http://localhost:3000/Users/", "", ""},
		{"unknown branch", "http://localhost:3000/unknown/", "", ""},
		{"no parameter inference", "http://localhost:3000/users/42/", "", ""},
		{"query", "http://localhost:3000/users?u", "", ""},
		{"fragment", "http://localhost:3000/users#u", "", ""},
		{"complete endpoint", "http://localhost:3000/users", "", ""},
		{"empty input", "", "", ""},
		{"curl", "curl http://local", "", ""},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := urlCompletionApp()
			s := app.current()
			s.url.SetText(tc.before + tc.after)
			s.url.CursorIndex.Set(uniseg.GraphemeClusterCount(tc.before))
			screen := sidebarScreen(tt, app, urlInputID)
			if tc.want == "" {
				if s.urlVars.state.Visible.Peek() {
					tt.Fatalf("unexpected popup for %q", tc.before)
				}
				return
			}
			if !s.urlVars.state.Visible.Peek() {
				tt.Fatalf("no popup for %q\n%s", tc.before, screen.renderer.ScreenText())
			}
			screen.pressKey(tt, "tab")
			if got := s.url.GetText(); got != tc.want {
				tt.Fatalf("completed %q, want %q", got, tc.want)
			}
			wantCursor := uniseg.GraphemeClusterCount(strings.Split(tc.want, "?")[0])
			if tc.after == "wrong?x=1" && s.url.CursorIndex.Peek() != wantCursor {
				tt.Fatalf("Unicode cursor = %d, want %d", s.url.CursorIndex.Peek(), wantCursor)
			}
		})
	}
}

func TestURLCompletionDismissalAndCollectionChanges(tt *testing.T) {
	app := urlCompletionApp()
	s := app.current()
	screen := sidebarScreen(tt, app, urlInputID)
	pasteURL(tt, screen, "http://localhost:3000/teams/")
	if got := strings.Count(screen.renderer.ScreenText(), "members"); got != 1 {
		tt.Fatalf("duplicate collection URLs should produce one members suggestion, got %d", got)
	}
	screen.pressKey(tt, "escape")
	screen.render()
	if s.urlVars.state.Visible.Peek() {
		tt.Fatal("dismissed popup reopened without editing")
	}
	pasteURL(tt, screen, "m")
	if !s.urlVars.state.Visible.Peek() {
		tt.Fatal("editing didn't reopen completion")
	}
	req := model.NewRequest()
	req.File, req.URL = "new.posting.yaml", "http://localhost:3000/teams/managers"
	if !app.storeRequest(req) {
		tt.Fatal("save failed")
	}
	screen.render()
	screen.pressKey(tt, "tab")
	if s.url.GetText() != req.URL {
		tt.Fatalf("saved URL wasn't available: %s", s.url.GetText())
	}
	app.deleteRequests([]string{req.File})
	s.url.SetText("http://localhost:3000/teams/ma")
	s.url.CursorEnd()
	pasteURL(tt, screen, "n")
	if s.urlVars.state.Visible.Peek() {
		tt.Fatal("deleted URL is still suggested")
	}
	app.replaceCollection(&model.Collection{Requests: []model.Request{req}})
	screen.render()
	if !s.urlVars.state.Visible.Peek() {
		tt.Fatal("disk reload didn't refresh suggestions")
	}
	screen.pressKey(tt, "tab")
	if s.url.GetText() != req.URL {
		tt.Fatal("reloaded URL wasn't completed")
	}
}

func TestURLCompletionPopupFitsDisplayedContent(tt *testing.T) {
	for _, width := range []int{130, 84} {
		for _, tc := range []struct {
			url  string
			want int
		}{
			{"http://localhost:3000/users/:id/", 10}, // settings + two padding cells
			{"${BASE_URL}/users/$BA", 36},            // variable + gap + value + padding
			{"http://localhost:3000/cafe\u0301/", 8}, // three double-width characters + padding
			{"http://localhost:3000/long/", width},   // long labels stay within the terminal
		} {
			app := urlCompletionApp()
			req := model.NewRequest()
			req.URL = "http://localhost:3000/long/" + strings.Repeat("a", 150)
			app.collection.Peek().Requests = append(app.collection.Peek().Requests, req)
			app.refreshTree()
			app.current().url.SetText(tc.url)
			app.current().url.CursorEnd()
			screen := newScreen(app, width, 27)
			screen.focusID(tt, urlInputID)
			popup := screen.renderer.TopFloat()
			if popup == nil {
				tt.Fatalf("no popup for %q", tc.url)
			}
			if popup.Width != tc.want {
				tt.Errorf("%d-column terminal, %q: popup width = %d, want %d", width, tc.url, popup.Width, tc.want)
			}
			if popup.X < 0 || popup.X+popup.Width > width {
				tt.Errorf("popup is outside the %d-column terminal: x=%d width=%d", width, popup.X, popup.Width)
			}
			if tc.url == "http://localhost:3000/users/:id/" {
				pasteURL(tt, screen, "p")
				if got := screen.renderer.TopFloat().Width; got != 7 {
					tt.Errorf("filtering to posts should shrink the popup to 7 cells, got %d", got)
				}
			}
		}
	}
}
