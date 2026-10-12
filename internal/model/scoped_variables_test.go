package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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
	req.Captures = map[string]string{"SESSION_ID": "data.id"}
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

func TestGeneratedVariablesAreFreshPerRequest(t *testing.T) {
	req := NewRequest()
	req.URL = "https://example.test/${ID}?id=${$uuid}"
	req.VariableScope = &VariableScope{Variables: map[string]string{"ID": "${$uuid}"}}
	req.Headers = []KeyValue{{Name: "Idempotency-Key", Value: "${$uuid}", Enabled: true}}
	req.Body = Body{Type: BodyRaw, Raw: `${$uuid}|$${$uuid}|$$uuid|${$unknown}`}
	before := time.Now()
	previous := ""
	for range 2 {
		variables, err := VariablesForRequest(req, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		values := Values(variables)
		id := values["$uuid"]
		if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) || id == previous {
			t.Fatalf("not a fresh UUID v4: %q (previous %q)", id, previous)
		}
		previous = id
		sent, err := Resolve(req, MapLookup(values))
		if err != nil {
			t.Fatal(err)
		}
		if sent.URL != "https://example.test/"+id+"?id="+id || sent.Headers[0].Value != id || sent.Body.Raw != id+"|${$uuid}|$uuid|${$unknown}" {
			t.Fatalf("generator wasn't shared across fields/templates or escaping changed: %+v", sent)
		}
		instant, err := time.Parse(time.RFC3339Nano, values["$isoTimestamp"])
		if err != nil || instant.Before(before) || instant.After(time.Now()) || !strings.HasSuffix(values["$isoTimestamp"], "Z") {
			t.Fatalf("invalid generated time: %q (%v)", values["$isoTimestamp"], err)
		}
		if values["$timestamp"] != strconv.FormatInt(instant.Unix(), 10) || values["$timestampMs"] != strconv.FormatInt(instant.UnixMilli(), 10) {
			t.Fatal("timestamp generators disagree on the send time")
		}
		n, err := strconv.ParseInt(values["$randomInt"], 10, 64)
		if err != nil || n < 0 || n > 2147483647 {
			t.Fatalf("invalid random integer: %q", values["$randomInt"])
		}
		req.Options.SubstituteBodyVariables = false
		literal, err := Resolve(req, MapLookup(values))
		if err != nil || literal.Body.Raw != req.Body.Raw || literal.Headers[0].Value != id {
			t.Fatal("body substitution option not respected")
		}
		req.Options.SubstituteBodyVariables = true
		for _, payload := range []Payload{
			GraphQL{Query: `query Q($id: ID!) { user(id: "${$uuid}") { id } }`, Variables: `{"id":"${ID}"}`},
			GRPC{Method: "users/Get", Message: `{"id":"${ID}"}`},
		} {
			req.Payload = payload
			wire, err := Resolve(req, MapLookup(values))
			if err != nil {
				t.Fatal(err)
			}
			switch p := wire.Payload.(type) {
			case GraphQL:
				if !strings.Contains(p.Query, id) || !strings.Contains(p.Query, "$id") || p.Variables != `{"id":"`+id+`"}` {
					t.Fatal(p)
				}
			case GRPC:
				if p.Message != `{"id":"`+id+`"}` {
					t.Fatal(p)
				}
			}
		}
		req.Payload = nil
	}
}
