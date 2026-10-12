package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

// Every kind must be sendable, by the real senders and by the fake the UI
// tests use.
func TestEveryKindHasASender(t *testing.T) {
	http, grpc := NewHTTP("", TLSSettings{}), NewGRPC("", TLSSettings{}, "")
	senders := ByKind{HTTP: http, GRPC: grpc}
	for _, k := range model.Kinds {
		t.Run(string(k.ID), func(t *testing.T) {
			sender := senders.For(k)
			if sender == nil {
				t.Fatalf("%s requests have no sender", k.Label)
			}
			if (sender == Sender(http)) != k.OverHTTP() {
				t.Errorf("%s requests go to %T", k.Label, sender)
			}
			if _, err := (Fake{}).Send(context.Background(), Call{Request: k.Example(), Variables: map[string]string{"BASE_URL": "http://x", "API_TOKEN": "t", "USER_ID": "7"}}); err != nil {
				t.Errorf("Fake can't send the example %s request: %v", k.Label, err)
			}
		})
	}
}

func TestByKindRoutesEachKindToItsSender(t *testing.T) {
	var got []string
	record := func(name string) Sender {
		return SenderFunc(func(context.Context, Call) (*model.Response, error) {
			got = append(got, name)
			return &model.Response{}, nil
		})
	}
	senders := ByKind{HTTP: record("http"), GRPC: record("grpc")}
	for _, k := range model.Kinds {
		if _, err := senders.Send(context.Background(), Call{Request: k.New()}); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"http", "http", "grpc"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sent to %v, want %v", got, want)
	}
	if _, err := (ByKind{HTTP: record("http")}).Send(context.Background(), Call{Request: model.GRPCKind.New()}); err == nil || !strings.Contains(err.Error(), "gRPC") {
		t.Fatalf("a kind without a sender: %v", err)
	}
}

func TestHTTPRefusesGRPC(t *testing.T) {
	req := grpcRequest("localhost:1", "a.B/C", "{}")
	if _, err := NewHTTP("", TLSSettings{}).Send(context.Background(), Call{Request: req}); err == nil || !strings.Contains(err.Error(), "aren't sent over HTTP") {
		t.Fatalf("err = %v", err)
	}
}

func TestFakeDescribesOneMethodOfEachShape(t *testing.T) {
	schema, err := Fake{}.Describe(context.Background(), Call{Request: model.GRPCKind.New()})
	if err != nil {
		t.Fatal(err)
	}
	shapes := map[Streaming]bool{}
	for _, m := range schema.Methods {
		shapes[m.Streaming] = true
		var template any
		if err := json.Unmarshal([]byte(m.Template), &template); err != nil {
			t.Errorf("%s's template isn't JSON: %v", m.Name, err)
		}
		if _, isArray := template.([]any); isArray != m.Streaming.clientStreams() {
			t.Errorf("%s (%s) template = %s", m.Name, m.Streaming, m.Template)
		}
	}
	if len(shapes) != 4 {
		t.Fatalf("shapes = %v", shapes)
	}
}

func TestFakeEchoesGRPCMessages(t *testing.T) {
	req := grpcRequest("localhost:50051", "posting.example.v1.Greeter/SayHello", `{"name": "${NAME}"}`)
	resp, err := Fake{}.Send(context.Background(), Call{Request: req, Variables: map[string]string{"NAME": "Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != `{"name": "Ada"}` || model.StatusOf(req, resp).Code != "OK" || resp.ContentType() != "application/json" {
		t.Fatalf("fake answered %+v", resp)
	}
	req.Payload = model.GRPC{Method: "posting.example.v1.Greeter/Missing"}
	resp, _ = Fake{}.Send(context.Background(), Call{Request: req})
	if model.StatusOf(req, resp).Code != "NOT_FOUND" {
		t.Fatalf("fake answered %+v", resp)
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
