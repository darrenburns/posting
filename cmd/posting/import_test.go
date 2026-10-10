package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/curl"
	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/darrenburns/posting/v3/internal/paths"
)

const postmanCLIExample = `{"info":{"name":"CLI collection","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[{"name":"Ping","request":{"method":"GET","url":"https://example.test/ping"}}]}`
const openapiCLIExample = `openapi: 3.1.0
info:
  title: CLI collection
  version: '1'
servers:
  - url: https://example.test
paths:
  /ping:
    get:
      summary: Ping
      responses:
        '200':
          description: OK
`
const brunoCLIExample = `meta {
  name: Ping
  type: http
  seq: 1
}

get {
  url: https://example.test/ping
  body: none
  auth: none
}
`

func TestImportCLIFormatsAndRepeatedImports(t *testing.T) {
	for _, tc := range []struct{ format, ext, data string }{{"postman", ".json", postmanCLIExample}, {"openapi", ".yaml", openapiCLIExample}, {"bruno", ".bru", brunoCLIExample}} {
		t.Run(tc.format, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input"+tc.ext)
			if err := os.WriteFile(input, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "output")
			for n := 1; n <= 2; n++ {
				var stdout, stderr bytes.Buffer
				args := []string{"import", input, "-o", output}
				if n == 2 {
					args = append(args, "--type", tc.format)
				}
				if code := run(args, &stdout, &stderr); code != 0 {
					t.Fatalf("exit %d: %s", code, stderr.String())
				}
				if !strings.Contains(stdout.String(), "Imported 1 "+tc.format+" request") {
					t.Fatalf("output: %s", stdout.String())
				}
				root, problems := (collection.Dir{Root: output}).Load()
				if len(problems) > 0 {
					t.Fatal(problems)
				}
				count := 0
				root.Walk(func(_ *model.Collection, req model.Request) {
					count++
					if req.URL != "https://example.test/ping" || req.Method != model.MethodGet {
						t.Errorf("request: %+v", req)
					}
				})
				if count != n {
					t.Fatalf("loaded %d requests after %d imports", count, n)
				}
			}
		})
	}
}

func TestImportCLIInvalidInputsDoNotWrite(t *testing.T) {
	for _, tc := range []struct{ name, data, format string }{{"unknown", `{"hello":"world"}`, ""}, {"invalid", "{bad json", "postman"}, {"empty", "", "openapi"}, {"unsupported", "swagger: '2.0'\ninfo: {}", "openapi"}} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(source, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "absent")
			var stdout, stderr bytes.Buffer
			args := []string{"import", "-o", output, source}
			if tc.format != "" {
				args = append(args, "-t", tc.format)
			}
			if code := run(args, &stdout, &stderr); code == 0 {
				t.Fatal("accepted invalid input")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("created output: %v", err)
			}
		})
	}
}

func TestImportCLIOptions(t *testing.T) {
	for _, args := range [][]string{{"input.json", "-o", "dest", "-t", "postman"}, {"-o=dest", "--type=postman", "input.json"}, {"-t", "postman", "--output", "dest", "--", "input.json"}} {
		got, err := parseImportOptions(args, &bytes.Buffer{})
		if err != nil || !reflect.DeepEqual(got, importOptions{sources: []string{"input.json"}, format: "postman", output: "dest"}) {
			t.Fatalf("%v => %+v, %v", args, got, err)
		}
	}
	got, err := parseImportOptions([]string{"collection.json", "-o", "dest", "staging.json", "production.json"}, &bytes.Buffer{})
	if err != nil || !reflect.DeepEqual(got.sources, []string{"collection.json", "staging.json", "production.json"}) {
		t.Fatalf("environments: %+v, %v", got, err)
	}
	for _, args := range [][]string{nil, {"--type", "unknown", "file"}, {"--output"}, {"--bad", "file"}} {
		if _, err := parseImportOptions(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"import", "--help"}, &out, &out); code != 0 || !strings.Contains(out.String(), "Bruno") {
		t.Fatalf("help: %d %s", code, out.String())
	}
}

func TestImportCLIDefaultDestination(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "collection.json")
	if err := os.WriteFile(source, []byte(postmanCLIExample), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", source}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	want := filepath.Join(paths.DefaultCollectionDir(), "CLI collection")
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
}

func TestImportCLIEnvironmentInstructions(t *testing.T) {
	input := strings.Replace(postmanCLIExample, `"item":`, `"variable":[{"key":"BASE_URL","value":"https://example.test"}],"item":`, 1)
	input = strings.Replace(input, "https://example.test/ping", "{{BASE_URL}}/ping", 1)
	source := filepath.Join(t.TempDir(), "collection.json")
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", source, "-o", output}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(stdout.String(), "Environments: posting.env (base)\n") || !strings.Contains(stdout.String(), " --env posting\n") {
		t.Fatalf("missing environment instructions: %s", stdout.String())
	}
}

func writeImportFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func loadImportedEnvironment(t *testing.T, dir, name string) map[string]string {
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

const postmanLayeredCollection = `{"info":{"name":"Layers","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
"variable":[{"key":"HOST","value":"https://prod.example"},{"key":"BASE_URL","value":"{{HOST}}/api"}],
"item":[{"name":"Ping","request":{"method":"GET","url":"{{BASE_URL}}/ping"}}]}`

func TestImportCLICollectionWithEnvironments(t *testing.T) {
	in := writeImportFiles(t, map[string]string{
		"collection.json":                     postmanLayeredCollection,
		"staging.postman_environment.json":    `{"name":"staging","values":[{"key":"HOST","value":"https://staging.example","enabled":true}],"_postman_variable_scope":"environment"}`,
		"production.postman_environment.json": `{"name":"production","values":[{"key":"TOKEN","value":"{{HOST}}-token","enabled":true}],"_postman_variable_scope":"environment"}`,
	})
	output := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	args := []string{"import", filepath.Join(in, "staging.postman_environment.json"), filepath.Join(in, "collection.json"), "-o", output, filepath.Join(in, "production.postman_environment.json")}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	want := "Imported 1 postman request(s) into " + strconv.Quote(output) + ".\nEnvironments: posting.env (base), staging.env, production.env\nOpen with: posting -c " + curl.Quote(output) + " --env staging\n"
	if stdout.String() != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout.String(), want)
	}
	for name, values := range map[string]map[string]string{
		"posting":    {"HOST": "https://prod.example", "BASE_URL": "https://prod.example/api"},
		"staging":    {"HOST": "https://staging.example", "BASE_URL": "https://staging.example/api"},
		"production": {"HOST": "https://prod.example", "BASE_URL": "https://prod.example/api", "TOKEN": "https://prod.example-token"},
	} {
		if got := loadImportedEnvironment(t, output, name); !reflect.DeepEqual(got, values) {
			t.Fatalf("%s: %q", name, got)
		}
	}
}

func TestImportCLIEnvironmentsOnly(t *testing.T) {
	in := writeImportFiles(t, map[string]string{
		"staging.postman_environment.json": `{"name":"staging","values":[{"key":"HOST","value":"https://staging.example"}]}`,
		"collection.json":                  postmanLayeredCollection,
		"other.json":                       postmanCLIExample,
	})
	environment := filepath.Join(in, "staging.postman_environment.json")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", environment}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "--output") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	output := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"import", filepath.Join(in, "collection.json"), "-o", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"import", environment, "-o", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Imported 1 environment(s)") || !strings.Contains(stdout.String(), "Environments: staging.env\n") || !strings.Contains(stdout.String(), "--env staging\n") {
		t.Fatalf("stdout: %s", stdout.String())
	}
	if got := loadImportedEnvironment(t, output, "staging")["BASE_URL"]; got != "https://staging.example/api" {
		t.Fatalf("an environment added later still layers on the collection's base: %q", got)
	}
	if code := run([]string{"import", filepath.Join(in, "collection.json"), filepath.Join(in, "other.json"), "-o", t.TempDir()}, &stdout, &stderr); code == 0 {
		t.Fatal("accepted two collections")
	}
}

func TestImportCLIBrunoDirectory(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "bruno.json"), []byte(`{"version":"1","name":"CLI collection","type":"collection"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "ping.bru"), []byte(brunoCLIExample), 0o600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", source, "-o", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	root, problems := (collection.Dir{Root: output}).Load()
	if len(problems) > 0 || len(root.Children) != 1 || len(root.Children[0].Requests) != 1 {
		t.Fatalf("collection: %+v, %v", root, problems)
	}
}

func TestImportCLIRejectsOversizedSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "huge.json")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((32 << 20) + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	output := filepath.Join(t.TempDir(), "absent")
	if code := run([]string{"import", source, "-o", output}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "32 MiB") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("created output: %v", err)
	}
}

func TestImportWarningsEscapeTerminalControls(t *testing.T) {
	var out bytes.Buffer
	printImportWarnings(&out, []string{"request \x1b[2J\nforged line"})
	if strings.Contains(out.String(), "\x1b") || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("unsafe diagnostic %q", out.String())
	}
}

func TestImportErrorsEscapeTerminalControls(t *testing.T) {
	source := filepath.Join(t.TempDir(), "collection.json")
	input := strings.Replace(postmanCLIExample, `"name":"Ping"`, `"name":"\u001b[2J\nforged line"`, 1)
	input = strings.Replace(input, `"method":"GET"`, `"method":"INVALID"`, 1)
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", source, "-o", t.TempDir()}, &stdout, &stderr); code == 0 {
		t.Fatal("expected failure")
	}
	if strings.Contains(stderr.String(), "\x1b") || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("unsafe diagnostic: %q", stderr.String())
	}
}

func TestImportCLIDetectsJSONOutsideYAMLCharacterSet(t *testing.T) {
	source := filepath.Join(t.TempDir(), "collection.json")
	input := strings.Replace(postmanCLIExample, `"method":"GET"`, `"method":"GET","description":"`+"\u008b"+`"`, 1)
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"import", source, "-o", t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("valid JSON rejected: %s", stderr.String())
	}
}
