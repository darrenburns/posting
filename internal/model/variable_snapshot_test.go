package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestVariableSnapshotFreezesTemplatesScopeOverridesHostAndGeneratedValues(t *testing.T) {
	req := GRPCKind.New()
	req.URL = "localhost:50051"
	req.Payload = GRPC{Method: "test.Service/Chat", Message: `{"id":"${$uuid}"}`}
	scope := &VariableScope{Variables: map[string]string{"WORD": "scope", "NESTED": "${ALIAS}", "ID": "${$uuid}"}}
	variables := []Variable{
		templateVariable("ALIAS", "${WORD}/${HOST_ONLY}/${LITERAL}"),
		{Name: "WORD", Value: "environment"},
		{Name: "LITERAL", Value: "${UNEXPANDED}$cash"},
		{Name: "WORD", Value: "session", SessionOverride: true},
		templateVariable("UNUSED", "${MISSING}"),
	}
	host := map[string]string{"HOST_ONLY": "host"}
	snapshot := NewVariableSnapshot(variables, scope, MapLookup(host))
	initial, err := snapshot.ForRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	id := Values(initial)["$uuid"]
	*variables[0].Template = "changed-template"
	variables[3].Value = "changed-session"
	scope.Variables["NESTED"] = "changed-scope"
	host["HOST_ONLY"] = "changed-host"
	host["MISSING"] = "added-later"
	got, err := snapshot.Substitute("${NESTED}|${ID}|${$uuid}|$${NESTED}|${UNKNOWN}")
	if err != nil {
		t.Fatal(err)
	}
	want := "session/host/${UNEXPANDED}$cash|" + id + "|" + id + "|${NESTED}|${UNKNOWN}"
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if _, err := snapshot.Substitute("${UNUSED}"); err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("missing-at-start fallback changed: %v", err)
	}
}

func TestVariableSnapshotMessageFailuresDoNotPoisonOtherMessages(t *testing.T) {
	variables := []Variable{
		templateVariable("MISSING_ALIAS", "${MISSING}"),
		templateVariable("CYCLE", "${CYCLE}"),
		templateVariable("LARGE", "${CHUNK}${CHUNK}"),
		{Name: "CHUNK", Value: strings.Repeat("x", (8<<20)+1)},
		templateVariable("REFERENCES", strings.Repeat("${EMPTY}", 10001)),
		{Name: "EMPTY", Value: ""},
		templateVariable("VALID", "${WORD}"),
		{Name: "WORD", Value: "works"},
	}
	for i := 0; i < 33; i++ {
		variables = append(variables, templateVariable(fmt.Sprintf("v%d", i), fmt.Sprintf("${v%d}", i+1)))
	}
	variables = append(variables, Variable{Name: "v33", Value: "too deep"})
	snapshot := NewVariableSnapshot(variables, nil, nil)
	req := GRPCKind.New()
	req.URL = "localhost:50051"
	if _, err := snapshot.ForRequest(req); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, errorText string }{
		{"${MISSING_ALIAS}", "MISSING"}, {"${CYCLE}", "cyclic"}, {"${LARGE}", "16 MiB"},
		{"${REFERENCES}", "10000"}, {"${v0}", "depth"},
	} {
		if _, err := snapshot.Substitute(tc.input); err == nil || !strings.Contains(err.Error(), tc.errorText) {
			t.Errorf("%s: %v", tc.input, err)
		}
		if got, err := snapshot.Substitute("${VALID}"); err != nil || got != "works" {
			t.Fatalf("recovery = %q, %v", got, err)
		}
	}
}

func TestVariableSnapshotSupportsConcurrentMessages(t *testing.T) {
	snapshot := NewVariableSnapshot([]Variable{templateVariable("ALIAS", "${VALUE}"), {Name: "VALUE", Value: "same"}}, nil, nil)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 10 {
				if got, err := snapshot.Substitute("${ALIAS}"); err != nil || got != "same" {
					t.Errorf("substitution = %q, %v", got, err)
				}
			}
		})
	}
	wg.Wait()
}
