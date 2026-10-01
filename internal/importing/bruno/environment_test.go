package bruno

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

func writeCollection(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// imported writes result as Posting would and resolves each request under
// the named environment.
func imported(t *testing.T, result importing.Result, environment string) (map[string]model.Request, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	if _, err := importing.Write(result, dir); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	var variables []model.Variable
	if files := env.Stack(dir, environment); files != nil {
		loaded, err := env.Load(files)
		if err != nil {
			t.Fatal(err)
		}
		variables = loaded.Variables
		values = model.Values(variables)
	} else if environment != env.BaseName {
		t.Fatalf("no environment %q", environment)
	}
	requests := map[string]model.Request{}
	for _, req := range result.Requests {
		scoped, err := model.VariablesForRequest(req, variables, os.LookupEnv)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := model.Resolve(req, model.MapLookup(model.Values(scoped)))
		if err != nil {
			t.Fatal(err)
		}
		requests[filepath.ToSlash(req.File)] = resolved
	}
	return requests, values
}

func TestEnvironmentsOverrideCollectionVariablesButNotFolderVariables(t *testing.T) {
	dir := writeCollection(t, map[string]string{
		"bruno.json":               `{"name":"Layers","type":"collection"}`,
		"collection.bru":           "vars:pre-request {\n  host: https://prod.example\n  baseUrl: {{host}}/api\n  who: collection\n}\n",
		"A/folder.bru":             "vars:pre-request {\n  who: folder\n  host: https://folder.example\n}\n",
		"A/one.bru":                "get {\n  url: {{baseUrl}}/{{who}}\n}\n",
		"B/two.bru":                "get {\n  url: {{baseUrl}}/{{who}}\n}\n",
		"environments/staging.bru": "vars {\n  host: https://staging.example\n  who: staging\n  ~off: x\n}\nvars:secret [\n  token,\n  ~disabledSecret\n]\n",
	})
	result, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Environments) != 1 || result.Environments[0].Name != "staging" {
		t.Fatalf("environments: %+v", result.Environments)
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "secret variable token has no value on disk; set it in staging.local.env") || strings.Contains(warnings, "disabledSecret") {
		t.Fatalf("warnings: %s", warnings)
	}
	for _, tc := range []struct{ environment, b string }{
		{"posting", "https://prod.example/api/collection"},
		{"staging", "https://staging.example/api/staging"},
	} {
		requests, values := imported(t, result, tc.environment)
		if got := requests["A/one.posting.yaml"].URL; got != "https://folder.example/api/folder" {
			t.Fatalf("%s: folder variables must win: %q", tc.environment, got)
		}
		if got := requests["B/two.posting.yaml"].URL; got != tc.b {
			t.Fatalf("%s: got %q, want %q", tc.environment, got, tc.b)
		}
		if _, ok := values["off"]; ok {
			t.Fatalf("%s: imported a disabled variable", tc.environment)
		}
	}
}

func TestEnvironmentFilesAreReadSafely(t *testing.T) {
	dir := writeCollection(t, map[string]string{
		"bruno.json":                   `{"name":"Envs","type":"collection"}`,
		"ping.bru":                     "get {\n  url: {{host}}/$literal\n}\n",
		"environments/b.bru":           "vars {\n  host: https://b.example/{{process.env.HOME_DIR}}/$5\n  bad-name: x\n}\n",
		"environments/a.bru":           "vars {\n  host: https://a.example\n}\n",
		"environments/notes.txt":       "not an environment",
		"environments/nested/c.bru":    "vars {\n  host: c\n}\n",
		"nested/environments/ping.bru": "get {\n  url: https://nested.example\n}\n",
	})
	outside := writeCollection(t, map[string]string{"evil.bru": "vars {\n  host: https://evil.example\n}\n"})
	if err := os.Symlink(filepath.Join(outside, "evil.bru"), filepath.Join(dir, "environments", "evil.bru")); err != nil {
		t.Skip(err)
	}
	result, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []importing.Environment{
		{Name: "a", Variables: []model.Variable{{Name: "host", Value: "https://a.example", Source: "bruno"}}},
		{Name: "b", Variables: []model.Variable{{Name: "host", Value: "https://b.example/${HOME_DIR}/$$5", Source: "bruno"}}},
	}
	if !reflect.DeepEqual(result.Environments, want) {
		t.Fatalf("environments: %+v", result.Environments)
	}
	if len(result.Requests) != 2 {
		t.Fatalf("a folder named environments below the root holds requests: %+v", result.Requests)
	}
	warnings := strings.Join(result.Warnings, "\n")
	for _, needle := range []string{"skipped symlink environments/evil.bru", "skipped environments/nested", "skipped environments/notes.txt", "bad-name"} {
		if !strings.Contains(warnings, needle) {
			t.Fatalf("missing warning %q: %s", needle, warnings)
		}
	}
	t.Setenv("HOME_DIR", "home")
	requests, _ := imported(t, result, "b")
	if got := requests["ping.posting.yaml"].URL; got != "https://b.example/home/$5/$literal" {
		t.Fatalf("url: %q", got)
	}
}

func TestListBlocks(t *testing.T) {
	d, err := parseDocument([]byte("vars:secret [\n  a,\n  ~b, c\n  d\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.list("vars:secret"); !reflect.DeepEqual(got, []string{"a", "c", "d"}) {
		t.Fatalf("list: %q", got)
	}
	if _, err := parseDocument([]byte("vars:secret [\n  a\n")); err == nil {
		t.Fatal("accepted an unclosed list")
	}
}

func TestScopedConstantsKeepBrunoJSONEscaping(t *testing.T) {
	for _, value := range []string{"a\"b", `a\b`, "first\nsecond"} {
		t.Run(value, func(t *testing.T) {
			literal := value
			if strings.Contains(literal, "\n") {
				literal = "'''\n    " + strings.ReplaceAll(literal, "\n", "\n    ") + "\n  '''"
			}
			files := map[string]string{
				"bruno.json":         `{}`,
				"folder/folder.bru":  "vars:pre-request {\n value: " + literal + "\n alias: {{value}}\n}\n",
				"folder/http.bru":    "post {\n url: https://example.test\n body: json\n}\nbody:json {\n {\"value\":\"{{alias}}\"}\n}\n",
				"folder/graphql.bru": "post {\n url: https://example.test/graphql\n body: graphql\n}\nbody:graphql {\n query Q($id: ID!) { user(id: $id) { name } }\n}\nbody:graphql:vars {\n {\"value\":\"{{alias}}\"}\n}\n",
			}
			result, err := Load(writeCollection(t, files))
			if err != nil {
				t.Fatal(err)
			}
			requests, _ := imported(t, result, "posting")
			for _, req := range requests {
				body := req.Body.Raw
				if g, ok := req.Payload.(model.GraphQL); ok {
					body = g.Variables
				}
				var got map[string]string
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatalf("invalid JSON %q: %v", body, err)
				}
				if got["value"] != value {
					t.Fatalf("JSON value %q, want %q", got["value"], value)
				}
			}
		})
	}
}
