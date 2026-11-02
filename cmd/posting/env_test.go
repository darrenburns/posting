package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestEnvironmentFilesAcceptNamesAndFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"posting.env":       "A=1\n",
		"staging.env":       "A=2\n",
		"staging.local.env": "A=3\n",
		"extra.env":         "B=1\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := func(files []string) []string {
		var out []string
		for _, f := range files {
			out = append(out, filepath.Base(f))
		}
		return out
	}
	dirs := []string{t.TempDir(), dir}

	got, err := environmentFiles([]string{"staging"}, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"posting.env", "staging.env", "staging.local.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("-e staging = %v, want %v", names(got), want)
	}

	// Names and files layer in order, and a file shared by two names (the
	// base) is used once, where it first appears.
	got, err = environmentFiles([]string{"staging", filepath.Join(dir, "extra.env"), "posting"}, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"posting.env", "staging.env", "staging.local.env", "extra.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("-e staging -e extra.env -e posting = %v, want %v", names(got), want)
	}

	// Repeating a file re-applies it: the last one given wins.
	extra, staging := filepath.Join(dir, "extra.env"), filepath.Join(dir, "staging.env")
	got, err = environmentFiles([]string{extra, staging, extra}, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"extra.env", "staging.env", "extra.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("-e extra.env -e staging.env -e extra.env = %v, want %v", names(got), want)
	}
	// ...even when a name included it first.
	got, err = environmentFiles([]string{"staging", filepath.Join(dir, "posting.env")}, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"posting.env", "staging.env", "staging.local.env", "posting.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("-e staging -e posting.env = %v, want %v", names(got), want)
	}

	// A file is used exactly as given, without the base.
	got, err = environmentFiles([]string{filepath.Join(dir, "staging.env")}, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"staging.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("-e staging.env = %v, want %v", names(got), want)
	}

	if _, err := environmentFiles([]string{"nowhere"}, dirs); err == nil {
		t.Error("an unknown environment should be an error")
	}

	// Without -e, the base environment in the working directory is used.
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	got, err = environmentFiles(nil, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"posting.env"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("default = %v, want %v", names(got), want)
	}
}

func TestNamedEnvironmentReappliesItsLayers(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"posting.env":       "X=base\nBASE_ONLY=base\n",
		"posting.local.env": "X=base-local\n",
		"staging.env":       "X=stage\n",
		"staging.local.env": "LOCAL=stage-local\n",
		"prod.env":          "X=prod\nBASE_ONLY=prod\nLOCAL=prod\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name     string
		args     []string
		baseOnly string
	}{
		{"file before name", []string{filepath.Join(dir, "staging.env"), "staging"}, "base"},
		{"repeated name", []string{"staging", "prod", "staging"}, "prod"},
		{"local file before name", []string{filepath.Join(dir, "staging.local.env"), "prod", "staging"}, "prod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := environmentFiles(tc.args, []string{dir})
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := env.Load(files)
			if err != nil {
				t.Fatal(err)
			}
			got := model.Values(loaded.Variables)
			want := map[string]string{"X": "stage", "LOCAL": "stage-local", "BASE_ONLY": tc.baseOnly}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("values = %v, want %v (files %v)", got, want, files)
			}
		})
	}
}

func TestSettingsInEnvironmentFilesAreReported(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"posting.env": "BASE_URL=https://example.com\n",
		"dev.env":     "POSTING_SSL__CA_BUNDLE=/dev-ca.pem\nTOKEN=abc\nexport posting_theme=lantern\n",
	}
	var paths []string
	for _, name := range []string{"posting.env", "dev.env"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(files[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	got := settingsInEnvironmentFiles(paths)
	want := []string{"dev.env sets POSTING_SSL__CA_BUNDLE, posting_theme, but settings aren't read from environment files. Set them in config.yaml or your shell instead."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
	if got := settingsInEnvironmentFiles(paths[:1]); got != nil {
		t.Errorf("warnings without settings = %q, want none", got)
	}
}
