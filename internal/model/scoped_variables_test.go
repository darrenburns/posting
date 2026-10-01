package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func templateVariable(name, template string) Variable {
	return Variable{Name: name, Value: template, Template: &template, Source: "import.env"}
}

func TestRequestVariablesUseFinalScopeAndKeepLiterals(t *testing.T) {
	req := NewRequest()
	req.URL = "${base}/ping"
	req.VariableScope = &VariableScope{Variables: map[string]string{"host": "folder.test"}}
	vars := []Variable{templateVariable("base", "https://${host}/${token}"), {Name: "token", Value: "${literal}$cash", Source: "staging.local.env"}, {Name: "host", Value: "session.test", Source: "session", SessionOverride: true}}
	resolved, err := VariablesForRequest(req, vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	sent, err := Resolve(req, MapLookup(Values(resolved)))
	if err != nil {
		t.Fatal(err)
	}
	if sent.URL != "https://session.test/${literal}$cash/ping" {
		t.Fatal(sent.URL)
	}
	if req.VariableScope.Variables["host"] != "folder.test" {
		t.Fatal("resolution changed the request scope")
	}
}

func TestRequestVariableErrorsOnlyFollowUsedDependencies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		variables []Variable
		want      string
	}{
		{"missing", []Variable{templateVariable("base", "https://example.test/${MISSING_SCOPED_TOKEN}")}, "MISSING_SCOPED_TOKEN"},
		{"cycle", []Variable{templateVariable("base", "${other}"), templateVariable("other", "${base}")}, "cyclic"},
		{"size", []Variable{templateVariable("base", "${large}${large}"), {Name: "large", Value: strings.Repeat("x", (8<<20)+1)}}, "16 MiB"},
		{"reference count", []Variable{templateVariable("base", strings.Repeat("${x}", 10001)), {Name: "x", Value: ""}}, "10000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequest()
			req.URL = "${base}"
			if _, err := VariablesForRequest(req, tc.variables, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want %s", err, tc.want)
			}
			req.URL = "https://independent.test"
			if _, err := VariablesForRequest(req, tc.variables, nil); err != nil {
				t.Fatalf("unused alias blocks request: %v", err)
			}
		})
	}
}

func TestVariableHostFallbackCanBeOverriddenBySession(t *testing.T) {
	req := NewRequest()
	req.URL = "${base}"
	vars := []Variable{templateVariable("base", "https://${host}")}
	host := func(name string) (string, bool) { return "host.test", name == "host" }
	loaded, err := ScopedVariables(vars, nil, host)
	if err != nil {
		t.Fatal(err)
	}
	if Values(loaded)["base"] != "https://host.test" {
		t.Fatal(loaded)
	}
	loaded = Merge(loaded, []Variable{{Name: "host", Value: "session.test", Source: "session", SessionOverride: true}})
	resolved, err := VariablesForRequest(req, loaded, host)
	if err != nil {
		t.Fatal(err)
	}
	if Values(resolved)["base"] != "https://session.test" {
		t.Fatal(resolved)
	}
}

func TestVariableScopeCloneIsIndependent(t *testing.T) {
	req := NewRequest()
	req.VariableScope = &VariableScope{Variables: map[string]string{"host": "original"}}
	clone := req.Clone()
	clone.VariableScope.Variables["host"] = "changed"
	if req.VariableScope.Variables["host"] != "original" {
		t.Fatal("clone shares variable scope")
	}
}

func TestGraphQLBindingsDoNotResolveEnvironmentTemplates(t *testing.T) {
	req := NewRequest()
	req.URL = "https://example.test/graphql"
	req.Payload = GraphQL{Query: "query Q($id: ID!) { user(id: $id) { name } }"}
	if _, err := VariablesForRequest(req, []Variable{templateVariable("id", "${id}")}, nil); err != nil {
		t.Fatal(err)
	}
	req.Payload = GraphQL{Query: "{ user(id: ${id}) { name } }"}
	if _, err := VariablesForRequest(req, []Variable{templateVariable("id", "${id}")}, nil); err == nil {
		t.Fatal("Posting reference did not resolve the cyclic template")
	}
}

func TestVariableTemplateDepthIsBounded(t *testing.T) {
	req := NewRequest()
	req.URL = "${v0}"
	var variables []Variable
	for i := 0; i < 33; i++ {
		variables = append(variables, templateVariable(fmt.Sprintf("v%d", i), fmt.Sprintf("${v%d}", i+1)))
	}
	variables = append(variables, Variable{Name: "v33", Value: "https://example.test"})
	if _, err := VariablesForRequest(req, variables, nil); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatal(err)
	}
}

func TestRequestScopeSurvivesHistoryJSON(t *testing.T) {
	req := NewRequest()
	req.URL = "${base}"
	req.VariableScope = &VariableScope{Variables: map[string]string{"base": "https://example.test"}}
	req.Payload = GraphQL{Query: "query Q($id: ID!) { user(id: $id) { name } }"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Request
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, loaded) {
		t.Fatalf("history round-trip changed scope: %s", data)
	}
}

func TestEnvironmentFileNamedSessionDoesNotOverrideRequestScope(t *testing.T) {
	req := NewRequest()
	req.URL = "${base}"
	req.VariableScope = &VariableScope{Variables: map[string]string{"host": "folder.test"}}
	variables := []Variable{templateVariable("base", "https://${host}"), {Name: "host", Value: "environment.test", Source: "session"}}
	resolved, err := VariablesForRequest(req, variables, nil)
	if err != nil {
		t.Fatal(err)
	}
	if Values(resolved)["base"] != "https://folder.test" {
		t.Fatal(resolved)
	}
}
