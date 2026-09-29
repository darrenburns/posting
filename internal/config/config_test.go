package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultsWithoutAFile(t *testing.T) {
	s, warnings := load(filepath.Join(t.TempDir(), "missing.yaml"), nil)
	if len(warnings) > 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	if !reflect.DeepEqual(s, Defaults()) {
		t.Fatalf("settings differ from defaults:\n%+v", s)
	}
}

func TestLoadsPosting2Config(t *testing.T) {
	s, warnings := load("../../tests/sample-configs/modified_config.yaml", nil)
	if len(warnings) > 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	if s.Layout != "horizontal" || s.Focus.OnStartup != "collection" || s.Focus.OnResponse != "body" {
		t.Errorf("layout/focus = %q %+v", s.Layout, s.Focus)
	}
	if s.Heading.Visible || s.Response.ShowSizeAndTime || s.WatchEnvFiles {
		t.Errorf("booleans not applied: %+v", s)
	}
	// Settings missing from the file keep their defaults.
	if !s.Response.PrettifyJSON || !s.Heading.ShowHost || s.CollectionBrowser.Position != "left" {
		t.Errorf("defaults lost: %+v", s)
	}
}

func TestEnvironmentOverridesTheFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(file, []byte("theme: nord\nheading:\n  show_host: true\nkeymap:\n  send-request: ctrl+r\n"), 0o644)
	s, warnings := load(file, []string{
		"POSTING_THEME=dracula",
		"POSTING_HEADING__SHOW_HOST=false",
		"POSTING_RESPONSE__PRETTIFY_JSON=0",
		"POSTING_NERD_FONTS=true",
		"POSTING_CONFIG_FILE=/elsewhere.yaml",
		"POSTING_EMPTY=",
		"UNRELATED=1",
	})
	if len(warnings) > 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	if s.Theme != "dracula" || s.Heading.ShowHost || s.Response.PrettifyJSON {
		t.Errorf("env overrides not applied: theme=%q heading=%+v response=%+v", s.Theme, s.Heading, s.Response)
	}
	if s.NerdFonts == nil || !*s.NerdFonts {
		t.Error("nerd_fonts should be set")
	}
	if got := s.KeysFor("send-request", "ctrl+j"); !reflect.DeepEqual(got, []string{"ctrl+r"}) {
		t.Errorf("keymap = %v", got)
	}
	if got := s.KeysFor("save-request", "ctrl+s"); !reflect.DeepEqual(got, []string{"ctrl+s"}) {
		t.Errorf("default keys = %v", got)
	}
}

func TestInvalidValuesFallBack(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(file, []byte("layout: diagonal\nfocus:\n  on_startup: nowhere\n"), 0o644)
	s, warnings := load(file, nil)
	if s.Layout != "vertical" || s.Focus.OnStartup != "url" || len(warnings) != 2 {
		t.Fatalf("layout=%q focus=%q warnings=%v", s.Layout, s.Focus.OnStartup, warnings)
	}

	os.WriteFile(file, []byte("layout: [not a string\n"), 0o644)
	s, warnings = load(file, nil)
	if !reflect.DeepEqual(s, Defaults()) || len(warnings) != 1 {
		t.Fatalf("unreadable file: warnings=%v", warnings)
	}
}
