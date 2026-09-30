package env

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func baseNames(files []string) []string {
	var out []string
	for _, f := range files {
		out = append(out, filepath.Base(f))
	}
	return out
}

func TestLoadRecordsOverridesAndUsesEarlierLayers(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"posting.env":       "BASE_URL=https://example.com\nTOKEN=base\n",
		"staging.env":       "BASE_URL=https://staging.example.com\nAPI=${BASE_URL}/v2\nUSERS=${TOKEN}-users\n",
		"staging.local.env": "TOKEN=secret\nSTILL=${API}\n",
	})
	e, err := Load(Stack(dir, "staging"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "staging" {
		t.Errorf("name = %q", e.Name)
	}
	byName := map[string]model.Variable{}
	for _, v := range e.Variables {
		byName[v.Name] = v
	}
	if v := byName["API"]; v.Value != "https://staging.example.com/v2" || v.Source != "staging.env" {
		t.Errorf("API = %+v: a layer should see its own earlier values", v)
	}
	if v := byName["USERS"]; v.Value != "base-users" {
		t.Errorf("USERS = %+v: a layer sees only the layers beneath it", v)
	}
	if v := byName["STILL"]; v.Value != "https://staging.example.com/v2" {
		t.Errorf("STILL = %+v", v)
	}
	if v := byName["TOKEN"]; v.Value != "secret" || v.Source != "staging.local.env" || !reflect.DeepEqual(v.Overrides, []string{"posting.env"}) {
		t.Errorf("TOKEN = %+v", v)
	}
	if v := byName["BASE_URL"]; v.Source != "staging.env" || !reflect.DeepEqual(v.Overrides, []string{"posting.env"}) {
		t.Errorf("BASE_URL = %+v", v)
	}
}

func TestStackFollowsTheConvention(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"posting.env":       "A=1\n",
		"posting.local.env": "A=2\n",
		"staging.env":       "A=3\n",
		"staging.local.env": "A=4\n",
		"prod.env":          "A=5\n",
		"only.local.env":    "A=6\n",
	})
	cases := map[string][]string{
		"staging":       {"posting.env", "posting.local.env", "staging.env", "staging.local.env"},
		"prod":          {"posting.env", "posting.local.env", "prod.env"},
		"only":          {"posting.env", "posting.local.env", "only.local.env"},
		"posting":       {"posting.env", "posting.local.env"},
		"":              {"posting.env", "posting.local.env"},
		"missing":       nil,
		"../prod":       nil,
		"posting.local": nil,
	}
	for name, want := range cases {
		got := Stack(dir, name)
		for _, f := range got {
			if !filepath.IsAbs(f) {
				t.Errorf("%s isn't absolute", f)
			}
		}
		if !reflect.DeepEqual(baseNames(got), want) {
			t.Errorf("Stack(%q) = %v, want %v", name, baseNames(got), want)
		}
	}
	if got := Stack(t.TempDir(), "posting"); got != nil {
		t.Errorf("an empty folder has no base: %v", got)
	}
	if got := Named([]string{t.TempDir(), dir}, "prod"); !reflect.DeepEqual(baseNames(got), cases["prod"]) {
		t.Errorf("Named = %v", got)
	}
	// Without a base, a named environment is just its own files.
	lone := t.TempDir()
	write(t, lone, map[string]string{"dev.env": "A=1\n"})
	if got := baseNames(Stack(lone, "dev")); !reflect.DeepEqual(got, []string{"dev.env"}) {
		t.Errorf("Stack without a base = %v", got)
	}
}

func TestName(t *testing.T) {
	cases := []struct {
		files []string
		want  string
	}{
		{nil, ""},
		{[]string{"/a/posting.env"}, "posting"},
		{[]string{"/a/posting.env", "/a/posting.local.env"}, "posting"},
		{[]string{"/a/posting.env", "/a/staging.env", "/a/staging.local.env"}, "staging"},
		{[]string{"/a/staging.local.env"}, "staging"},
		{[]string{"/a/dev.env"}, "dev"},
		{[]string{"/a/.env"}, ".env"},
		{[]string{"/a/.env.test"}, ".env.test"},
		{[]string{"/a/one.env", "/a/two.env"}, "one.env + two.env"},
	}
	for _, c := range cases {
		if got := Name(c.files); got != c.want {
			t.Errorf("Name(%v) = %q, want %q", c.files, got, c.want)
		}
	}
}

func TestCandidatesGroupLayers(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"posting.env":       "A=1\n",
		"posting.local.env": "A=2\n",
		"staging.env":       "A=3\n",
		"staging.local.env": "A=4\n",
		"prod.env":          "A=5\n",
		".env":              "A=6\n",
	})
	other := t.TempDir()
	write(t, other, map[string]string{"dev.env": "A=7\n"})
	var got []string
	for _, files := range (Source{Dirs: []string{dir, other, dir}}).Candidates() {
		got = append(got, Name(files)+"="+strings.Join(baseNames(files), ","))
	}
	want := []string{
		"posting=posting.env,posting.local.env",
		".env=.env",
		"prod=posting.env,posting.local.env,prod.env",
		"staging=posting.env,posting.local.env,staging.env,staging.local.env",
		"dev=dev.env",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates =\n%v\nwant\n%v", got, want)
	}
}

func TestMemory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"posting.env": "A=1\n", "staging.env": "A=2\n"})
	m := Memory{File: filepath.Join(t.TempDir(), "state", "environments.json")}
	if got := m.Recall("/c"); got != nil {
		t.Fatalf("nothing remembered yet: %v", got)
	}
	files := Stack(dir, "staging")
	if err := m.Remember("/c", files); err != nil {
		t.Fatal(err)
	}
	if err := m.Remember("/other", []string{filepath.Join(dir, "posting.env")}); err != nil {
		t.Fatal(err)
	}
	if got := m.Recall("/c"); !reflect.DeepEqual(got, files) {
		t.Fatalf("Recall = %v, want %v", got, files)
	}
	os.Remove(filepath.Join(dir, "staging.env"))
	if got := m.Recall("/c"); got != nil {
		t.Fatalf("an environment with a missing file isn't recalled: %v", got)
	}
	if err := m.Remember("/c", nil); err != nil {
		t.Fatal(err)
	}
	if got := m.Recall("/other"); len(got) != 1 {
		t.Fatalf("forgetting one collection should keep the other: %v", got)
	}
}
