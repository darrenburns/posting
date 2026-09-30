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
	vars := []model.Variable{{Name: "TOKEN", Value: "it's \\ ${HOME} \"quoted\"\nwith\r\nnewlines"}, {Name: "EMPTY"}}
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
	loaded, err := env.Load([]string{filepath.Join(dir, result.Environment)})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, v := range loaded.Variables {
		values[v.Name] = v.Value
	}
	for _, v := range vars {
		if values[v.Name] != v.Value {
			t.Fatalf("variable %s changed: %q != %q", v.Name, values[v.Name], v.Value)
		}
	}
	again, err := Write(Result{Variables: vars}, dir)
	if err != nil || again.Environment != "imported-2.env" {
		t.Fatalf("repeat import: %+v %v", again, err)
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
	for _, s := range []string{"", "normal", "'quote'", `C:\files\`, "${HOME}", "line\r\nnext", "x\x00y"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		data, err := environmentData([]model.Variable{{Name: "VALUE", Value: value}})
		if err != nil {
			t.Fatal(err)
		}
		pairs := env.Parse(string(data), func(string) (string, bool) { return "unexpected host value", true })
		if len(pairs) != 1 || pairs[0].Value != value {
			t.Fatalf("roundtrip %q => %q", value, pairs)
		}
	})
}
