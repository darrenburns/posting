package ui

import (
	"errors"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

// flakySource serves the sample environments and fails for unknown files.
type flakySource struct{ StaticEnvironments }

func (f flakySource) Load(files []string) (model.Environment, error) {
	for _, e := range f.StaticEnvironments {
		if envKey(e.Files) == envKey(files) {
			return e, nil
		}
	}
	return model.Environment{}, errors.New("no such file")
}

func TestSwitchingEnvironmentsKeepsSessionVariables(tt *testing.T) {
	app := New(Config{
		Collection:    model.SampleCollection(),
		Environments:  flakySource{StaticEnvironments(model.SampleEnvironments())},
		Environment:   []string{"local.env"},
		HostVariables: []model.Variable{{Name: "HOME_ONLY", Value: "h", Source: "host"}, {Name: "BASE_URL", Value: "host-base", Source: "host"}},
		UserHost:      "user@host",
	})
	if got := app.variableValuesPeek()["BASE_URL"]; got != "http://localhost:8000" {
		tt.Fatalf("environment should override the host: BASE_URL = %q", got)
	}
	app.sessionVars.Set(map[string]string{"API_TOKEN": "mine"})

	app.switchEnvironment([]string{"staging.env"})
	values := app.variableValuesPeek()
	if values["BASE_URL"] != "https://staging.example.com/api" || values["API_TOKEN"] != "mine" || values["HOME_ONLY"] != "h" {
		tt.Fatalf("after switching: %v", values)
	}
	if app.envName() != "staging" {
		tt.Fatalf("env name = %q", app.envName())
	}

	// A failed switch leaves the environment alone.
	app.switchEnvironment([]string{"gone.env"})
	if app.envName() != "staging" || app.toast.Peek().kind != toastError {
		tt.Fatalf("failed switch changed the environment to %q", app.envName())
	}

	app.switchEnvironment(nil)
	values = app.variableValuesPeek()
	if _, ok := values["PASSWORD"]; ok || values["API_TOKEN"] != "mine" || values["BASE_URL"] != "host-base" {
		tt.Fatalf("with no environment: %v", values)
	}

	// Environments used this session come first, then the rest, then none.
	items := app.environmentItems()
	var labels []string
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	want := []string{"local", "staging", "No environment"}
	if len(labels) != len(want) {
		tt.Fatalf("items = %v", labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			tt.Fatalf("items = %v, want %v", labels, want)
		}
	}
	if !items[2].Current {
		tt.Fatal("No environment should be marked current")
	}
}
