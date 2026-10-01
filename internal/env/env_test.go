package env

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	input := `# comment
BASE_URL=https://example.com   # trailing comment
export TOKEN=abc123
EMPTY=
QUOTED="hello # not a comment"
SINGLE='literal ${BASE_URL}'
ESCAPED="line1\nline2 \"q\""
MULTI="first
second"
EXPANDED=${BASE_URL}/v1
DEFAULTED=${NOPE:-fallback}
FROM_HOST=${HOME_DIR}
not an assignment
1BAD=x
`
	host := func(name string) (string, bool) {
		if name == "HOME_DIR" {
			return "/home/ada", true
		}
		return "", false
	}
	got := Parse(input, host)
	want := []Pair{
		{"BASE_URL", "https://example.com"},
		{"TOKEN", "abc123"},
		{"EMPTY", ""},
		{"QUOTED", "hello # not a comment"},
		{"SINGLE", "literal ${BASE_URL}"},
		{"ESCAPED", "line1\nline2 \"q\""},
		{"MULTI", "first\nsecond"},
		{"EXPANDED", "https://example.com/v1"},
		{"DEFAULTED", "fallback"},
		{"FROM_HOST", "/home/ada"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse =\n%q\nwant\n%q", got, want)
	}
}

func TestLoadLayersFiles(t *testing.T) {
	base := "../../tests/sample-envs/sample_base.env"
	extra := "../../tests/sample-envs/sample_extra.env"
	e, err := Load([]string{base, extra})
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "sample_base.env + sample_extra.env" {
		t.Errorf("name = %q", e.Name)
	}
	values := map[string]string{}
	sources := map[string]string{}
	for _, v := range e.Variables {
		values[v.Name] = v.Value
		sources[v.Name] = v.Source
	}
	if values["FILE"] != "extra" || values["POST_ID"] != "2" || values["ONLY_BASE"] != "true" || values["ONLY_EXTRA"] != "true" {
		t.Errorf("values = %v", values)
	}
	if sources["FILE"] != "sample_extra.env" || sources["USER_ID"] != "sample_base.env" {
		t.Errorf("sources = %v", sources)
	}
	if _, err := Load([]string{"missing.env"}); err == nil {
		t.Error("loading a missing file should fail")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".env", "dev.env", ".env.local", "notes.txt", "env"} {
		os.WriteFile(filepath.Join(dir, name), []byte("A=1\n"), 0o644)
	}
	os.Mkdir(filepath.Join(dir, "folder.env"), 0o755)
	got := Discover([]string{dir, dir})
	var names []string
	for _, path := range got {
		names = append(names, filepath.Base(path))
	}
	if want := []string{".env", ".env.local", "dev.env"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Discover = %v, want %v", names, want)
	}
}

func TestTemplatesKeepReferences(t *testing.T) {
	data := "A=plain $x # comment\n" +
		"export B='single ${A} $5'\n" +
		"C=\"${A}/${B} $D ${:-$}{literal} ${:-}x\"\n" +
		"D=\"${A:-fallback}\"\n" +
		"E=\"${my-var}\"\n" +
		"F=\"multi\nline ${A}\"\n"
	pairs, skipped := Templates(data)
	want := []Pair{
		{Name: "A", Value: "plain $$x"},
		{Name: "B", Value: "single $${A} $$5"},
		{Name: "C", Value: "${A}/${B} $$D $${literal} x"},
		{Name: "F", Value: "multi\nline ${A}"},
	}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("Templates = %q, want %q", pairs, want)
	}
	if !reflect.DeepEqual(skipped, []string{"D", "E"}) {
		t.Fatalf("skipped = %q", skipped)
	}
}
