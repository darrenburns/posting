package ui

import (
	"errors"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
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

func TestSwitchingRemembersTheEnvironment(tt *testing.T) {
	var remembered [][]string
	app := New(Config{
		Collection:          model.SampleCollection(),
		Environments:        flakySource{StaticEnvironments(model.SampleEnvironments())},
		Environment:         []string{"local.env"},
		RememberEnvironment: func(files []string) { remembered = append(remembered, files) },
		UserHost:            "user@host",
	})
	if len(remembered) != 0 {
		tt.Fatalf("the startup environment isn't a choice to remember: %v", remembered)
	}
	app.switchEnvironment([]string{"staging.env"})
	app.switchEnvironment([]string{"gone.env"})
	app.switchEnvironment(nil)
	want := [][]string{{"staging.env"}, nil}
	if len(remembered) != len(want) || remembered[0][0] != "staging.env" || remembered[1] != nil {
		tt.Fatalf("remembered %v, want %v", remembered, want)
	}
}

func TestVariablesShowWhatTheyOverride(tt *testing.T) {
	app := New(Config{
		Collection: model.SampleCollection(),
		Environments: StaticEnvironments{{Name: "staging", Files: []string{"posting.env", "staging.env"}, Variables: []model.Variable{
			{Name: "BASE_URL", Value: "https://staging", Source: "staging.env", Overrides: []string{"posting.env"}},
			{Name: "TIMEOUT", Value: "5", Source: "posting.env"},
		}}},
		Environment:   []string{"posting.env", "staging.env"},
		HostVariables: []model.Variable{{Name: "TIMEOUT", Value: "1", Source: "host"}},
		UserHost:      "user@host",
	})
	app.sessionVars.Set(map[string]string{"BASE_URL": "http://mine"})
	got := map[string]string{}
	for _, v := range app.variableList() {
		got[v.Name] = variableSource(v)
	}
	if got["BASE_URL"] != "session over staging.env" || got["TIMEOUT"] != "posting.env over host" {
		tt.Fatalf("sources = %v", got)
	}
	if v := app.variableValuesPeek(); v["BASE_URL"] != "http://mine" || v["TIMEOUT"] != "5" {
		tt.Fatalf("values = %v", v)
	}
}
