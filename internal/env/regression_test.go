package env

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// base.env is an ordinary environment called "base", not the base itself.
func TestBaseEnvIsAnOrdinaryEnvironment(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"posting.env": "A=1\n", "base.env": "A=2\n"})
	var got []string
	for _, files := range (Source{Dirs: []string{dir}}).Candidates() {
		got = append(got, Name(files)+"="+strings.Join(baseNames(files), ","))
	}
	if want := []string{"posting=posting.env", "base=posting.env,base.env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("with posting.env: Candidates = %v, want %v", got, want)
	}

	// Without a base, base.env is offered on its own, not as a blank entry.
	lone := t.TempDir()
	write(t, lone, map[string]string{"base.env": "A=2\n"})
	got = nil
	for _, files := range (Source{Dirs: []string{lone}}).Candidates() {
		if len(files) == 0 {
			t.Fatal("Candidates offered an environment with no files")
		}
		got = append(got, Name(files)+"="+strings.Join(baseNames(files), ","))
	}
	if want := []string{"base=base.env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("without posting.env: Candidates = %v, want %v", got, want)
	}
}

// A file layered twice applies its values again where it reappears.
func TestLoadReappliesARepeatedFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"a.env": "X=a\n", "b.env": "X=b\n"})
	a, b := filepath.Join(dir, "a.env"), filepath.Join(dir, "b.env")
	e, err := Load([]string{a, b, a})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Variables) != 1 || e.Variables[0].Value != "a" || e.Variables[0].Source != "a.env" {
		t.Fatalf("Variables = %+v, want X=a from a.env", e.Variables)
	}
}

// A remembered named environment is rebuilt, so layers added since count.
func TestMemoryRebuildsNamedEnvironments(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"posting.env": "A=1\n", "staging.env": "A=2\n", "extra.env": "A=3\n"})
	m := Memory{File: filepath.Join(t.TempDir(), "environments.json")}
	if err := m.Remember("/c", Stack(dir, "staging")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, map[string]string{"staging.local.env": "A=4\n"})
	if got, want := baseNames(m.Recall("/c")), []string{"posting.env", "staging.env", "staging.local.env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Recall = %v, want %v", got, want)
	}

	// Files composed by hand are recalled exactly as they were.
	composed := []string{filepath.Join(dir, "staging.env"), filepath.Join(dir, "extra.env")}
	if err := m.Remember("/c", composed); err != nil {
		t.Fatal(err)
	}
	if got := m.Recall("/c"); !reflect.DeepEqual(got, composed) {
		t.Fatalf("Recall = %v, want %v", got, composed)
	}

	// A named environment whose files have all gone isn't recalled.
	if err := m.Remember("/c", Stack(dir, "staging")); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "staging.env"))
	os.Remove(filepath.Join(dir, "staging.local.env"))
	if got := m.Recall("/c"); got != nil {
		t.Fatalf("Recall = %v after staging was deleted", got)
	}
}
