package importing

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/model"
)

func sampleRequest(file string) model.Request {
	r := model.NewRequest()
	r.Name, r.File, r.URL = "Example", file, "https://example.test/"
	return r
}

func TestWritePreservesBodiesVariablesAndExistingFiles(t *testing.T) {
	dir := t.TempDir()
	original := []byte("existing request")
	if err := os.WriteFile(filepath.Join(dir, "example.posting.yaml"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	r := sampleRequest("example.posting.yaml")
	r.Body = model.Body{Type: model.BodyRaw, ContentType: "text/plain", Raw: "keep spaces  \n\tand tabs\t\n"}
	vars := []model.Variable{{Name: "TOKEN", Value: "it's \\ $${HOME} \"quoted\"\nwith\r\nnewlines"}, {Name: "EMPTY"}}
	result, err := Write(Result{Requests: []model.Request{r, r}, Variables: vars}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Files, []string{"example-2.posting.yaml", "example-3.posting.yaml"}) {
		t.Fatalf("files: %v", result.Files)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "example.posting.yaml"))
	if string(data) != string(original) {
		t.Fatal("overwrote original")
	}
	for _, path := range result.Files {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		req, err := collection.ParseRequest(data, path)
		if err != nil {
			t.Fatal(err)
		}
		if req.Body.Raw != r.Body.Raw {
			t.Fatalf("body changed: %q", req.Body.Raw)
		}
	}
	if !reflect.DeepEqual(result.Environments, []WrittenEnvironment{{Name: "posting", File: "posting.env", Base: true}}) {
		t.Fatalf("environments: %+v", result.Environments)
	}
	values := loadEnvironment(t, dir, "posting")
	want := map[string]string{"TOKEN": "it's \\ ${HOME} \"quoted\"\nwith\r\nnewlines", "EMPTY": ""}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("loaded %q, want %q", values, want)
	}
	again, err := Write(Result{Variables: vars}, dir)
	if err != nil || len(again.Environments) != 1 || again.Environments[0].File != "posting-2.env" {
		t.Fatalf("repeat import: %+v %v", again, err)
	}
	if !strings.Contains(strings.Join(again.Warnings, "\n"), "posting.env already exists") {
		t.Fatalf("no warning that the base moved: %q", again.Warnings)
	}
	if got := loadEnvironment(t, dir, "posting"); !reflect.DeepEqual(got, want) {
		t.Fatalf("repeat import changed the base: %q", got)
	}
}

func TestWriteConfinesHostileNames(t *testing.T) {
	dir := t.TempDir()
	var requests []model.Request
	for _, name := range []string{"../../escape", `C:\outside\escape`, "/absolute/name", "CON", "NUL.txt", "a/./../b", strings.Repeat("long", 100), "a\x00b", "same?", "same*"} {
		requests = append(requests, sampleRequest(name))
	}
	got, err := Write(Result{Requests: requests}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != len(requests) {
		t.Fatal(got)
	}
	for _, name := range got.Files {
		if !filepath.IsLocal(name) {
			t.Fatalf("escaped: %q", name)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWriteRejectsSymlinkEscapeAndRollsBack(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
		t.Skip(err)
	}
	_, err := Write(Result{Requests: []model.Request{sampleRequest("first"), sampleRequest("escape/second")}}, dir)
	if err == nil {
		t.Fatal("accepted symlink escape")
	}
	if _, err := os.Stat(filepath.Join(dir, "first.posting.yaml")); !os.IsNotExist(err) {
		t.Fatalf("partial request remained: %v", err)
	}
	files, _ := os.ReadDir(outside)
	if len(files) != 0 {
		t.Fatal("wrote outside destination")
	}
}

func TestWriteValidatesBeforeCreatingOutput(t *testing.T) {
	for _, result := range []Result{{}, {Requests: []model.Request{sampleRequest("valid")}, Variables: []model.Variable{{Name: "BAD\nKEY", Value: "x"}}}} {
		dir := filepath.Join(t.TempDir(), "not-created")
		if _, err := Write(result, dir); err == nil {
			t.Fatal("expected error")
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("created output on invalid input: %v", err)
		}
	}
}

func FuzzEnvironmentRoundTrip(f *testing.F) {
	for _, s := range []string{"", "normal", "'quote'", `C:\files\`, "${HOME}", "$${HOME}", "line\r\nnext", "x\x00y", `${A}/$${B}/"q"\`, "${A}${:-x}$", "$$${A}{", "${A}$${", "${A} ${a b} ${", "$A$B", "${A}\n# not a comment"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, template string) {
		data, err := environmentData([]model.Variable{{Name: "VALUE", Value: template}})
		if err != nil {
			t.Fatal(err)
		}
		// No variable can have the empty name, which the literal "${" encoding
		// relies on.
		lookup := func(name string) (string, bool) { return "<" + name + " ${x} $$ \\\">", name != "" }
		pairs := env.Parse(string(data), lookup)
		want := model.Substitute(template, lookup)
		if len(pairs) != 1 || pairs[0].Value != want {
			t.Fatalf("%q encoded as %q loads as %q, want %q", template, data, pairs, want)
		}
		templates, skipped := env.Templates(string(data))
		if len(templates) != 1 || len(skipped) != 0 || model.Substitute(templates[0].Value, lookup) != want {
			t.Fatalf("%q encoded as %q reads back as template %q, skipped %q", template, data, templates, skipped)
		}
	})
}

func loadEnvironment(t *testing.T, dir, name string) map[string]string {
	t.Helper()
	files := env.Stack(dir, name)
	if files == nil {
		t.Fatalf("no environment %q in %s", name, dir)
	}
	loaded, err := env.Load(files)
	if err != nil {
		t.Fatal(err)
	}
	return model.Values(loaded.Variables)
}

func TestWriteLayersReferencesAcrossEnvironments(t *testing.T) {
	dir := t.TempDir()
	result := Result{
		Variables: []model.Variable{
			{Name: "API", Value: "${BASE_URL}/v1"},
			{Name: "BASE_URL", Value: "${SCHEME}://${HOST}/api"},
			{Name: "SCHEME", Value: "https"},
			{Name: "HOST", Value: "prod.test"},
			{Name: "UNRELATED", Value: "${SCHEME}"},
		},
		Environments: []Environment{
			{Name: "staging", Variables: []model.Variable{{Name: "DOCS", Value: "${API}/docs"}, {Name: "HOST", Value: "staging.test"}}},
			{Name: "production", Variables: []model.Variable{{Name: "EXTRA", Value: "x"}}},
		},
	}
	written, err := Write(result, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Warnings) > 0 {
		t.Fatalf("warnings: %q", written.Warnings)
	}
	want := []WrittenEnvironment{{Name: "posting", File: "posting.env", Base: true}, {Name: "staging", File: "staging.env"}, {Name: "production", File: "production.env"}}
	if !reflect.DeepEqual(written.Environments, want) {
		t.Fatalf("environments: %+v", written.Environments)
	}
	base := map[string]string{"API": "https://prod.test/api/v1", "BASE_URL": "https://prod.test/api", "SCHEME": "https", "HOST": "prod.test", "UNRELATED": "https"}
	if got := loadEnvironment(t, dir, "posting"); !reflect.DeepEqual(got, base) {
		t.Fatalf("base: %q", got)
	}
	staging := map[string]string{"API": "https://staging.test/api/v1", "BASE_URL": "https://staging.test/api", "SCHEME": "https", "HOST": "staging.test", "UNRELATED": "https", "DOCS": "https://staging.test/api/v1/docs"}
	if got := loadEnvironment(t, dir, "staging"); !reflect.DeepEqual(got, staging) {
		t.Fatalf("staging: %q", got)
	}
	production := map[string]string{"EXTRA": "x"}
	for k, v := range base {
		production[k] = v
	}
	if got := loadEnvironment(t, dir, "production"); !reflect.DeepEqual(got, production) {
		t.Fatalf("production: %q", got)
	}
}

func TestWriteRoundTripsAwkwardValuesBesideReferences(t *testing.T) {
	t.Setenv("IMPORT_TEST_HOST", "host value must not leak")
	dir := t.TempDir()
	value := "a $$ b $${x} c ${IMPORT_TEST_HOST} 'single' \"double\" back\\slash\\\nline\r\n$${ $$${IMPORT_TEST_HOST}"
	_, err := Write(Result{
		Variables:    []model.Variable{{Name: "IMPORT_TEST_HOST", Value: "base"}, {Name: "V", Value: value}},
		Environments: []Environment{{Name: "staging", Variables: []model.Variable{{Name: "IMPORT_TEST_HOST", Value: "${x"}}}},
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, host := range map[string]string{"posting": "base", "staging": "${x"} {
		got := loadEnvironment(t, dir, name)["V"]
		want := model.Substitute(value, model.MapLookup(map[string]string{"IMPORT_TEST_HOST": host}))
		if got != want {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestWriteKeepsReservedEnvironmentNamesOutOfTheBase(t *testing.T) {
	dir := t.TempDir()
	result := Result{
		Variables: []model.Variable{{Name: "WHO", Value: "base"}},
		Environments: []Environment{
			{Name: "Posting", Variables: []model.Variable{{Name: "WHO", Value: "posting env"}}},
			{Name: "staging.local", Variables: []model.Variable{{Name: "WHO", Value: "staging.local env"}}},
			{Name: "staging", Variables: []model.Variable{{Name: "WHO", Value: "staging env"}}},
		},
	}
	written, err := Write(result, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []WrittenEnvironment{{Name: "posting", File: "posting.env", Base: true}, {Name: "Posting-environment", File: "Posting-environment.env"}, {Name: "staging-local", File: "staging-local.env"}, {Name: "staging", File: "staging.env"}}
	if !reflect.DeepEqual(written.Environments, want) {
		t.Fatalf("environments: %+v", written.Environments)
	}
	for name, who := range map[string]string{"posting": "base", "Posting-environment": "posting env", "staging-local": "staging.local env", "staging": "staging env"} {
		if got := loadEnvironment(t, dir, name)["WHO"]; got != who {
			t.Fatalf("%s: WHO = %q, want %q", name, got, who)
		}
	}
}

func TestWriteEnvironmentsWithoutRequestsOrBase(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(Result{Environments: []Environment{{Name: "staging", Variables: []model.Variable{{Name: "HOST", Value: "staging.test"}}}, {Name: "empty"}}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Files) != 0 || len(written.Environments) != 2 {
		t.Fatalf("written: %+v", written)
	}
	if _, err := os.Stat(filepath.Join(dir, "posting.env")); !os.IsNotExist(err) {
		t.Fatalf("wrote a base without base variables: %v", err)
	}
	if got := loadEnvironment(t, dir, "staging"); !reflect.DeepEqual(got, map[string]string{"HOST": "staging.test"}) {
		t.Fatalf("staging: %q", got)
	}
	if got := loadEnvironment(t, dir, "empty"); len(got) != 0 {
		t.Fatalf("empty: %q", got)
	}
}

func TestLayersOrderReferencesAndReportCycles(t *testing.T) {
	vars := []model.Variable{
		{Name: "WAITS", Value: "${A}"},
		{Name: "A", Value: "${B}"},
		{Name: "B", Value: "${A}"},
		{Name: "SELF", Value: "${SELF}"},
		{Name: "LATE", Value: "x"},
		{Name: "USES_LATE", Value: "${LATE}"},
	}
	files, warnings := layers(vars, nil)
	var names []string
	for _, v := range files[0] {
		names = append(names, v.Name)
	}
	if want := []string{"SELF", "LATE", "USES_LATE", "A", "WAITS", "B"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("order %v, want %v", names, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "A, B reference each other") {
		t.Fatalf("warnings: %q", warnings)
	}
}

func FuzzImportPathsStayLocal(f *testing.F) {
	for _, s := range []string{"", ".", "..", "../../escape", `C:\Users\test`, "/absolute", "\x00name", "CON.txt", "folder/😀 request"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		path := safePath(strings.TrimSuffix(name, collection.FileSuffix)) + collection.FileSuffix
		if !filepath.IsLocal(path) {
			t.Fatalf("non-local output from %q: %q", name, path)
		}
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\x00\\:") {
				t.Fatalf("unsafe component %q", part)
			}
		}
	})
}

func FuzzImportedBodyPersistence(f *testing.F) {
	for _, s := range []string{"", "  trailing  \n", "\r\n", "\t\n", "${TOKEN}", "binary\x00body"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		req := sampleRequest("body.posting.yaml")
		req.Body = model.Body{Type: model.BodyRaw, Raw: body, ContentType: "text/plain"}
		req.Options.SubstituteBodyVariables = false
		data, err := collection.MarshalRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := collection.ParseRequest(data, req.File)
		if err != nil {
			t.Fatalf("%v; encoded YAML: %q", err, data)
		}
		if loaded.Body.Raw != body || loaded.Options.SubstituteBodyVariables {
			t.Fatalf("payload changed: %q => %q", body, loaded.Body.Raw)
		}
	})
}
