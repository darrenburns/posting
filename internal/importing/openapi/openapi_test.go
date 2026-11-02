package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestPetstore(t *testing.T) {
	data, err := os.ReadFile("testdata/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Pet Store" || len(result.Requests) != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	get, patch, form := result.Requests[0], result.Requests[1], result.Requests[2]
	if get.Name != "Find pet" || get.Description != "Returns one pet." || get.File != "Pets/Find pet.posting.yaml" || get.URL != "https://eu.example.test/v1/pets/:id" {
		t.Fatalf("GET metadata: %+v", get)
	}
	if get.PathParams[0].Value != "a/b" || get.Query[0].Value != "fr" || len(get.Query) != 3 {
		t.Fatalf("GET parameters: %+v", get)
	}
	if get.Auth.Type != model.AuthBearer || patch.Auth.Type != model.AuthNone || patch.URL != "https://edit.example.test/v2/pets/:id" {
		t.Fatalf("precedence: %+v %+v", get, patch)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(patch.Body.Raw), &body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body["id"]; exists {
		t.Fatal("readOnly property included")
	}
	if body["name"] != "Kitty $cat" || body["active"] != false {
		t.Fatalf("body: %#v", body)
	}
	if len(form.Body.Form) != 3 || form.Body.Type != model.BodyForm || form.Body.Form[0].Value != "Kitty $cat" {
		t.Fatalf("form: %+v", form.Body)
	}
	for _, req := range result.Requests {
		serialized, err := collection.MarshalRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		back, err := collection.ParseRequest(serialized, req.File)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, req) {
			t.Errorf("roundtrip changed request\nbefore: %#v\nafter:  %#v", req, back)
		}
	}
	// Repeated imports, including warning order and placeholder names, are stable.
	again, err := Parse(data)
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatalf("non-deterministic conversion: %v", err)
	}
}

func TestOnWire(t *testing.T) {
	var receivedURI, header, cookie, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURI = r.RequestURI
		header = r.Header.Get("X-Literal")
		cookie = r.Header.Get("Cookie")
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(204)
	}))
	defer server.Close()
	document := fmt.Sprintf(`{"openapi":"3.0.4","info":{"title":"wire"},"servers":[{"url":%q}],"paths":{"/pets/{id}":{"post":{"parameters":[{"name":"id","in":"path","example":"a/b"},{"name":"q","in":"query","example":["a,b","x&y"]},{"name":"X-Literal","in":"header","example":"$literal"},{"name":"session","in":"cookie","example":"a b"}],"requestBody":{"content":{"application/json":{"example":{"message":"$literal","number":9007199254740993}}}}}}}}`, server.URL)
	result, err := Parse([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	req := result.Requests[0]
	encoded, err := collection.MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	req, err = collection.ParseRequest(encoded, req.File)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.NewHTTP("import-test", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req, Variables: map[string]string{"literal": "wrong"}})
	if err != nil {
		t.Fatal(err)
	}
	if receivedURI != "/pets/a%2Fb?q=a%2Cb&q=x%26y" || header != "$literal" || cookie != "session=a+b" {
		t.Fatalf("wire: %q, %q, %q", receivedURI, header, cookie)
	}
	if !strings.Contains(body, `9007199254740993`) || !strings.Contains(body, `"$literal"`) {
		t.Fatalf("body changed: %s", body)
	}
}

func TestLocalReferences(t *testing.T) {
	data := []byte(`openapi: 3.1.0
info: {title: refs}
paths:
  /x:
    get:
      parameters:
      - {$ref: '#/components/parameters/a~0b~1c'}
      - {$ref: '#/components/parameters/space%20name'}
      - {$ref: '#/x-array/0'}
components:
  parameters:
    a~b/c: {name: a, in: query, example: one}
    space name: {name: b, in: query, example: two}
x-array:
- {name: c, in: query, example: three}
`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	query := result.Requests[0].Query
	if len(query) != 3 || query[0].Value != "one" || query[1].Value != "two" || query[2].Value != "three" {
		t.Fatalf("refs: %+v", query)
	}
	for _, ref := range []string{"#/missing", "#/components/parameters/a~2b", "https://example.test/param.yaml", "#/components/parameters/cycle", "#anchor", "#/x-array/01"} {
		t.Run(ref, func(t *testing.T) {
			document := fmt.Sprintf(`{"openapi":"3.0.3","paths":{"/x":{"get":{"parameters":[{"$ref":%q}]}}},"components":{"parameters":{"cycle":{"$ref":"#/components/parameters/cycle"}}},"x-array":[{},{}]}`, ref)
			if _, err := Parse([]byte(document)); err == nil {
				t.Fatal("expected reference error")
			}
		})
	}
}

func TestDiagnostics(t *testing.T) {
	cases := []struct{ name, operation, want string }{
		{"multipart", `"requestBody":{"content":{"multipart/form-data":{"example":{"file":"x"}}}}`, "multipart bodies"},
		{"delimited", `"parameters":[{"name":"tags","in":"query","explode":false,"example":["a","b"]}]`, "unsupported form serialization"},
		{"reserved", `"parameters":[{"name":"tags","in":"query","allowReserved":true,"example":"a/b"}]`, "allowReserved"},
		{"oauth", `"security":[{"oauth":[]}]`, "authentication needs manual"},
		{"schema", `"requestBody":{"content":{"application/json":{"schema":{"oneOf":[{"type":"string"},{"type":"number"}]}}}}`, "oneOf requires"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			document := `{"openapi":"3.1.0","servers":[{"url":"https://example.test"}],"paths":{"/x":{"post":{` + test.operation + `}}},"components":{"securitySchemes":{"oauth":{"type":"oauth2"}}}}`
			result, err := Parse([]byte(document))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(result.Warnings, "\n"), test.want) {
				t.Fatalf("missing warning %q: %v", test.want, result.Warnings)
			}
		})
	}
}

func TestMalformedAndUnsupported(t *testing.T) {
	for _, data := range []string{"", "[]", "null", `{"swagger":"2.0","paths":{}}`, `{"openapi":"3.2.0","paths":{}}`, `{"openapi":"3.1.0","paths":[]}`, "openapi: 3.1.0\npaths: {}\n---\npaths: {}", "openapi: 3.1.0\npaths: {}\npaths: {}", `{"openapi":"3.1.0","paths":{"/x":{"get":null}}}`, `{"openapi":"3.1.0","paths":{"/x":{"get":{"parameters":"bad"}}}}`, "openapi: 3.1.0\npaths: {}\nx: {1: value}"} {
		t.Run(data, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSecurityAlternativesAndServerScopes(t *testing.T) {
	data := []byte(`openapi: 3.1.0
servers: [{url: https://root.test}]
security: [{bearer: []}]
paths:
  /a:
    servers: [{url: https://path.test}]
    get: {security: [{}]}
    post: {servers: [], security: [{oauth: []}, {key: []}]}
  /b:
    get: {security: [{key: [], other: []}]}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
    oauth: {type: oauth2}
    key: {type: apiKey, in: query, name: key}
    other: {type: apiKey, in: header, name: X-Key}
`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].URL != "https://path.test/a" || result.Requests[0].Auth.Type != model.AuthNone {
		t.Fatal(result.Requests[0])
	}
	if result.Requests[1].URL != "${BASE_URL}/a" || len(result.Requests[1].Query) != 1 {
		t.Fatal(result.Requests[1])
	}
	if len(result.Requests[2].Query) != 1 || len(result.Requests[2].Headers) != 1 {
		t.Fatal(result.Requests[2])
	}
}

func TestRecursiveSchemaBounded(t *testing.T) {
	result, err := Parse([]byte(`{"openapi":"3.1.0","paths":{"/x":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}}}}},"components":{"schemas":{"Node":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/Node"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "recursive") {
		t.Fatal(result.Warnings)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{`{"openapi":"3.1.0","paths":{}}`, "openapi: 3.0.3\npaths: {/x: {get: {}}}", `{"openapi":"3.1.0","paths":{"/x":{"get":{"parameters":[{"$ref":"#"}]}}}}`, "a: &a [*a]"} {
		f.Add([]byte(seed))
	}
	fixture, _ := os.ReadFile("testdata/petstore.yaml")
	f.Add(fixture)
	f.Add([]byte(`{"openapi":"3.1.1","servers":[{"url":"https://example.test"}],"paths":{"/x":{"post":{"requestBody":{"content":{"application/json; charset=utf-8":{"example":{"n":18446744073709551617}}}}}}}}`))
	f.Add([]byte("openapi: 3.1.0\npaths: {/x: {get: {parameters: [{name: n, in: query, example: 1.00000000000000000001}]}}}\nx: &x {n: 2}\ny: {<<: *x}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := Parse(data)
		if err != nil {
			return
		}
		again, err := Parse(data)
		if err != nil || !reflect.DeepEqual(result, again) {
			t.Fatal("non-deterministic parser")
		}
		for _, req := range result.Requests {
			encoded, err := collection.MarshalRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			back, err := collection.ParseRequest(encoded, req.File)
			if err != nil {
				t.Fatal(err)
			}
			// Check the request that would actually be sent, not just that the
			// generated YAML is syntactically readable.
			lookup := model.MapLookup(map[string]string{"BASE_URL": "https://example.test"})
			beforeWire, beforeErr := model.Resolve(req, lookup)
			afterWire, afterErr := model.Resolve(back, lookup)
			if fmt.Sprint(beforeErr) != fmt.Sprint(afterErr) || !reflect.DeepEqual(beforeWire, afterWire) {
				t.Fatalf("persisted request changes resolution: before=%+v after=%+v", beforeWire, afterWire)
			}
		}
	})
}

func TestYAMLDateAndBooleanSchema(t *testing.T) {
	result, err := Parse([]byte(`openapi: 3.1.0
paths:
  /x:
    post:
      parameters: [{name: date, in: query, example: 2026-09-30}]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                free: true
                prohibited: false
                date: {default: 2026-09-30}
`))
	if err != nil {
		t.Fatal(err)
	}
	req := result.Requests[0]
	if req.Query[0].Value != "2026-09-30" || !strings.Contains(req.Body.Raw, `"2026-09-30"`) {
		t.Fatalf("dates changed: %+v", req)
	}
}

func TestReferencePlusIsNotSpace(t *testing.T) {
	result, err := Parse([]byte(`{"openapi":"3.1.0","paths":{"/x":{"get":{"parameters":[{"$ref":"#/components/parameters/a+b"}]}}},"components":{"parameters":{"a+b":{"name":"test","in":"query","example":"ok"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].Query[0].Value != "ok" {
		t.Fatal(result)
	}
}

// Set POSTING_OPENAPI_OFFICIAL_FIXTURES to a directory containing the downloaded
// upstream OAI and Swagger fixtures. Normal tests stay independent of networks.
func TestOfficialFixtures(t *testing.T) {
	dir := os.Getenv("POSTING_OPENAPI_OFFICIAL_FIXTURES")
	if dir == "" {
		t.Skip("optional downloaded upstream fixtures")
	}
	for _, name := range []string{"posting-openapi-oai-petstore.yaml", "posting-openapi-swagger-petstore.yaml"} {
		data, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(result.Requests) == 0 {
			t.Fatalf("%s: no requests", name)
		}
		t.Logf("%s: %d requests, %d diagnostics", name, len(result.Requests), len(result.Warnings))
	}
}
