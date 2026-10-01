package postman

import (
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

const layeredCollection = `{"info":{"name":"Layers"},
"variable":[{"key":"HOST","value":"https://prod.example"},{"key":"BASE_URL","value":"{{HOST}}/api"}],
"item":[
  {"name":"Users","request":{"method":"GET","url":"{{BASE_URL}}/users"}},
  {"name":"Folder","variable":[{"key":"SCOPED","value":"{{HOST}}/scoped"},{"key":"PRICE","value":"$5"}],"item":[
    {"name":"Scoped","request":{"method":"GET","url":"{{SCOPED}}?price={{PRICE}}"}}
  ]}
]}`

const stagingEnvironment = `{"name":"staging","values":[
  {"key":"HOST","value":"https://staging.example","type":"default","enabled":true},
  {"key":"OFF","value":"x","enabled":false}
],"_postman_variable_scope":"environment"}`

func resolveUnder(t *testing.T, dir, environment string, req model.Request) (model.Request, map[string]string) {
	t.Helper()
	loaded, err := env.Load(env.Stack(dir, environment))
	if err != nil {
		t.Fatal(err)
	}
	values := model.Values(loaded.Variables)
	resolved, err := model.Resolve(req, model.MapLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	return resolved, values
}

func TestCollectionReferencesFollowTheSelectedEnvironment(t *testing.T) {
	result, err := Parse([]byte(layeredCollection))
	if err != nil {
		t.Fatal(err)
	}
	staging, warnings, err := ParseEnvironment([]byte(stagingEnvironment))
	if err != nil || len(warnings) > 0 {
		t.Fatal(err, warnings)
	}
	result.Environments = append(result.Environments, staging)
	dir := t.TempDir()
	if _, err := importing.Write(result, dir); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ environment, host string }{{"posting", "https://prod.example"}, {"staging", "https://staging.example"}} {
		users, values := resolveUnder(t, dir, tc.environment, result.Requests[0])
		if values["BASE_URL"] != tc.host+"/api" || users.URL != tc.host+"/api/users" {
			t.Fatalf("%s: BASE_URL=%q URL=%q", tc.environment, values["BASE_URL"], users.URL)
		}
		if _, ok := values["OFF"]; ok {
			t.Fatalf("%s: imported a disabled value", tc.environment)
		}
		scoped, _ := resolveUnder(t, dir, tc.environment, result.Requests[1])
		if scoped.URL != tc.host+"/scoped?price=%245" {
			t.Fatalf("%s: folder variables: URL=%q", tc.environment, scoped.URL)
		}
	}
}

func TestParseEnvironment(t *testing.T) {
	got, warnings, err := ParseEnvironment([]byte(`{"name":"Local","values":[
		{"key":"PORT","value":8080},
		{"key":"DEBUG","value":true,"type":"default"},
		{"key":"PRICE","value":"$5 {{PORT}}"},
		{"key":"PRICE","value":"$6 {{PORT}}"},
		{"key":"bad-name","value":"x"},
		{"key":"EMPTY","value":null,"enabled":true},
		{"key":"SKIPPED","value":"x","enabled":false}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := importing.Environment{Name: "Local", Variables: []model.Variable{
		{Name: "PORT", Value: "8080", Source: "postman"},
		{Name: "DEBUG", Value: "true", Source: "postman"},
		{Name: "PRICE", Value: "$$6 ${PORT}", Source: "postman"},
		{Name: "EMPTY", Value: "", Source: "postman"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"bad-name"`) {
		t.Fatalf("warnings: %q", warnings)
	}
}

func TestParseEnvironmentRejectsGlobalsAndCollections(t *testing.T) {
	for _, data := range []string{
		`{"name":"Globals","values":[],"_postman_variable_scope":"globals"}`,
		`{"info":{"name":"Collection"},"item":[]}`,
		`[]`,
	} {
		_, _, err := ParseEnvironment([]byte(data))
		if err == nil {
			t.Fatalf("accepted %s", data)
		}
		if strings.Contains(data, "globals") && !strings.Contains(err.Error(), "globals") {
			t.Fatalf("unclear error: %v", err)
		}
	}
}
