package client

import (
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestGRPCOpenStreamResolvesAliasesFirstUsedLater(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/Chat", `{"text":"hi"}`)
	req.VariableScope = &model.VariableScope{Variables: map[string]string{"ALIAS": "${WORD}"}}
	snapshot := model.NewVariableSnapshot([]model.Variable{{Name: "WORD", Value: "there"}}, req.VariableScope, nil)
	variables, err := snapshot.ForRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	u := newUpdates()
	stream, result, _ := openStreamCall(t, Call{Request: req, Variables: model.Values(variables), ResolveBody: snapshot.Substitute}, u)
	u.waitFor(t, 1)
	if err := stream.Send(`{"text":"${ALIAS}"}`); err != nil {
		t.Fatal(err)
	}
	u.waitFor(t, 2)
	stream.Close()
	resp := <-result
	assertJSON(t, resp.Body, `[{"text":"HI"},{"text":"THERE"}]`)
}

func TestGRPCOpenStreamRejectsOnlyMessageWithInvalidAlias(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/Chat", `{"text":"first"}`)
	req.VariableScope = &model.VariableScope{Variables: map[string]string{
		"BAD": "${MISSING}", "CYCLE": "${CYCLE}", "GOOD": "${WORD}",
	}}
	snapshot := model.NewVariableSnapshot([]model.Variable{{Name: "WORD", Value: "second"}}, req.VariableScope, nil)
	variables, err := snapshot.ForRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	u := newUpdates()
	stream, result, _ := openStreamCall(t, Call{Request: req, Variables: model.Values(variables), ResolveBody: snapshot.Substitute}, u)
	u.waitFor(t, 1)
	for _, message := range []string{`{"text":"${BAD}"}`, `{"text":"${CYCLE}"}`} {
		if err := stream.Send(message); err == nil {
			t.Fatalf("invalid alias accepted in %s", message)
		}
	}
	if err := stream.Send(`{"text":"${GOOD}"}`); err != nil {
		t.Fatal(err)
	}
	u.waitFor(t, 2)
	stream.Close()
	resp := <-result
	assertJSON(t, resp.Body, `[{"text":"FIRST"},{"text":"SECOND"}]`)
	assertMessageLog(t, resp, 2, 2, model.MessageInfo)
}

func TestGRPCOpenStreamKeepsSubstitutionDisabled(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/Chat", `{"text":"${BAD}"}`)
	req.Options.SubstituteBodyVariables = false
	req.VariableScope = &model.VariableScope{Variables: map[string]string{"BAD": "${MISSING}"}}
	snapshot := model.NewVariableSnapshot(nil, req.VariableScope, nil)
	variables, err := snapshot.ForRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	u := newUpdates()
	stream, result, _ := openStreamCall(t, Call{Request: req, Variables: model.Values(variables), ResolveBody: snapshot.Substitute}, u)
	u.waitFor(t, 1)
	if err := stream.Send(`{"text":"$${BAD}"}`); err != nil {
		t.Fatal(err)
	}
	u.waitFor(t, 2)
	stream.Close()
	assertJSON(t, (<-result).Body, `[{"text":"${BAD}"},{"text":"$${BAD}"}]`)
}
