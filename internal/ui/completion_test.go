package ui

import "testing"

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
