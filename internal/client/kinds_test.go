package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

// Every kind must be sendable. Until a kind has its own transport, that
// means every kind is carried over HTTP.
func TestEveryKindHasASender(t *testing.T) {
	for _, k := range model.Kinds {
		if !k.OverHTTP() {
			t.Errorf("%s requests have no sender", k.Label)
		}
	}
}

func TestHTTPSendsGraphQLAsAJSONPost(t *testing.T) {
	type received struct {
		method, path, contentType, accept, auth, body string
	}
	got := make(chan received, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{r.Method, r.URL.RequestURI(), r.Header.Get("Content-Type"), r.Header.Get("Accept"), r.Header.Get("Authorization"), string(body)}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":null,"errors":[{"message":"nope"}]}`)
	}))
	defer server.Close()

	req := model.GraphQLKind.New()
	req.URL = "${BASE}/graphql?trace=1"
	req.Auth = model.Auth{Type: model.AuthBearer, Token: "${TOKEN}"}
	req.Payload = model.GraphQL{
		Query:         "query User($id: ID!) { user(id: $id, tag: \"${TAG}\") { name } }",
		Variables:     "{\"id\": \"${ID}\"}",
		OperationName: "User",
	}
	vars := map[string]string{"BASE": server.URL, "TOKEN": "t0k", "TAG": "x", "ID": "42", "id": "LEAKED"}
	resp := send(t, NewHTTP("posting-test", TLSSettings{}), req, vars)

	r := <-got
	want := received{
		method:      "POST",
		path:        "/graphql?trace=1",
		contentType: "application/json",
		accept:      "application/graphql-response+json, application/json",
		auth:        "Bearer t0k",
		body:        `{"query":"query User($id: ID!) { user(id: $id, tag: \"x\") { name } }","variables":{"id":"42"},"operationName":"User"}`,
	}
	if r != want {
		t.Fatalf("server received\n%+v\nwant\n%+v", r, want)
	}
	if status := model.StatusOf(req, resp); status != (model.Status{Code: "200", Text: "1 error", Class: model.StatusClassWarning}) {
		t.Fatalf("status = %+v", status)
	}
}

func TestFakeEchoesGraphQLBody(t *testing.T) {
	req := model.GraphQLKind.New()
	req.URL = "http://api.test/graphql"
	req.Payload = model.GraphQL{Query: "{ whoami }", Variables: `{"a": 1}`}
	resp, err := Fake{}.Send(context.Background(), Call{Request: req})
	if err != nil {
		t.Fatal(err)
	}
	var echo struct {
		Method string         `json:"method"`
		JSON   map[string]any `json:"json"`
	}
	if err := json.Unmarshal(resp.Body, &echo); err != nil {
		t.Fatal(err)
	}
	if echo.Method != "POST" || echo.JSON["query"] != "{ whoami }" || echo.JSON["variables"].(map[string]any)["a"] != 1.0 {
		t.Fatalf("fake echoed %s", resp.Body)
	}
}
