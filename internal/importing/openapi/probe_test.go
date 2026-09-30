package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

func TestProbeMediaTypesOnWire(t *testing.T) {
	for _, media := range []string{"application/json; charset=utf-8", "Application/JSON", "application/problem+json; charset=utf-8", "application/x-www-form-urlencoded; charset=utf-8"} {
		t.Run(media, func(t *testing.T) {
			var body, contentType string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				body = string(b)
				contentType = r.Header.Get("Content-Type")
				w.WriteHeader(204)
			}))
			defer server.Close()
			data := fmt.Sprintf(`{"openapi":"3.1.0","servers":[{"url":%q}],"paths":{"/x":{"post":{"requestBody":{"content":{%q:{"example":{"a":"$value"}}}}}}}}`, server.URL, media)
			result, err := Parse([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			req := persistedProbeRequest(t, result.Requests[0])
			_, err = client.NewHTTP("probe", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req})
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(media, "application/x-www-form-urlencoded") {
				if contentType != media {
					t.Fatalf("media type lost: %q", contentType)
				}
				if body != "a=%24value" {
					t.Fatalf("form body lost: %q warnings=%v", body, result.Warnings)
				}
			} else if !strings.Contains(body, `"a": "$value"`) {
				t.Fatalf("JSON body lost: %q contentType=%q warnings=%v", body, contentType, result.Warnings)
			}
		})
	}
}

func TestProbeExactJSONNumbers(t *testing.T) {
	for _, number := range []string{"18446744073709551617", "1.00000000000000000001", "1e400"} {
		t.Run(number, func(t *testing.T) {
			data := `{"openapi":"3.1.0","paths":{"/x":{"post":{"requestBody":{"content":{"application/json":{"example":{"number":` + number + `}}}}}}}}`
			result, err := Parse([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			req := persistedProbeRequest(t, result.Requests[0])
			resolved, err := model.Resolve(req, model.MapLookup(map[string]string{"BASE_URL": "https://example.test"}))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(resolved.Body.Raw, `"number": `+number) {
				t.Fatalf("number changed: %s", resolved.Body.Raw)
			}
		})
	}
}

func TestProbeArrayReferenceIndex(t *testing.T) {
	for _, index := range []string{"+0", "-0", "+1"} {
		data := fmt.Sprintf(`{"openapi":"3.1.0","paths":{"/x":{"get":{"parameters":[{"$ref":"#/x-params/%s"}]}}},"x-params":[{"name":"a","in":"query","example":"yes"},{"name":"b","in":"query","example":"no"}]}`, index)
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("accepted invalid JSON pointer index %q", index)
		}
	}
}

func TestProbeExactYAMLNumbers(t *testing.T) {
	data := `openapi: 3.1.0
paths:
  /x:
    post:
      requestBody:
        content:
          application/json:
            example:
              integer: 18446744073709551617
              decimal: 1.00000000000000000001
              exponent: 1e400
              hex: 0xff
              hugehex: 0x10000000000000001
              leadingzero: 01.00000000000000000001
              signedexponent: +1e400
              quoted: '1e400'
              explicit: !!str 1e400
              merged: {<<: &number {n: 1.00000000000000000001}, overridden: 2}
              alias: *number
`
	result, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	body := persistedProbeRequest(t, result.Requests[0]).Body.Raw
	for _, want := range []string{`"integer": 18446744073709551617`, `"decimal": 1.00000000000000000001`, `"exponent": 1e400`, `"hex": 255`, `"n": 1.00000000000000000001`, `"hugehex": 18446744073709551617`, `"leadingzero": 1.00000000000000000001`, `"signedexponent": 1e400`, `"quoted": "1e400"`, `"explicit": "1e400"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s: %s", want, body)
		}
	}
}

func TestProbeFormObjectEncoding(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.1"} {
		for _, encoding := range []string{`{}`, `{"address":{"style":"form"}}`, `{"address":{"style":"form","contentType":"application/json","headers":{"X-Ignored":{"example":"ignored"}}}}`} {
			t.Run(version+encoding, func(t *testing.T) {
				var received url.Values
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					received = r.PostForm
					w.WriteHeader(204)
				}))
				defer server.Close()
				data := fmt.Sprintf(`{"openapi":%q,"servers":[{"url":%q}],"paths":{"/x":{"post":{"requestBody":{"content":{"application/x-www-form-urlencoded":{"example":{"address":{"zip":"99999+1234"}},"encoding":%s}}}}}}}`, version, server.URL, encoding)
				result, err := Parse([]byte(data))
				if err != nil {
					t.Fatal(err)
				}
				form := result.Requests[0].Body.Form
				name, value := "address", `{"zip":"99999+1234"}`
				if encoding != `{}` {
					name, value = "zip", "99999+1234"
				}
				if len(form) != 1 || form[0].Name != name || form[0].Value != value {
					t.Fatalf("wrong form serialization: %+v warnings=%v", form, result.Warnings)
				}

				req := persistedProbeRequest(t, result.Requests[0])
				if _, err := client.NewHTTP("probe", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req}); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(received, url.Values{name: []string{value}}) {
					t.Fatalf("wrong wire form: %#v", received)
				}
			})
		}
	}
}

func TestProbeLiteralServerColon(t *testing.T) {
	data := []byte(`{"openapi":"3.1.1","servers":[{"url":"https://example.test/:id"}],"paths":{"/{id}":{"get":{"parameters":[{"in":"path","name":"id","example":"42"}]}}}}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	req, err := model.Resolve(result.Requests[0], model.MapLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://example.test/:id/42" {
		t.Fatalf("literal server path changed: %s", req.URL)
	}
}

func TestProbeReferencedBooleanSchemas(t *testing.T) {
	data := []byte(`{"openapi":"3.1.1","paths":{"/x":{"post":{"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"allowed":{"$ref":"#/components/schemas/Any"},"prohibited":{"$ref":"#/components/schemas/Never"},"known":{"default":"ok"}}}}}}}}},"components":{"schemas":{"Any":true,"Never":false}}}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatalf("valid boolean reference failed: %v", err)
	}
	if result.Requests[0].Body.Raw != "{\n  \"known\": \"ok\"\n}" {
		t.Fatal(result.Requests[0].Body.Raw)
	}
}

// Keep mutations inside a valid OpenAPI envelope so serialization semantics,
// rather than just document rejection, are exercised on every fuzz iteration.
func FuzzParameterRoundtrip(f *testing.F) {
	for _, value := range []string{"a/b", "$id&plus+ space", "中🙂%2f", "false", "\x00\n", ":id", "a,b=c;d"} {
		f.Add("$query", value)
	}
	f.Fuzz(func(t *testing.T, name, value string) {
		if name == "" || value == "" || len(name) > 256 || len(value) > 4096 || !utf8.ValidString(name) || !utf8.ValidString(value) {
			t.Skip()
		}
		parameter := object{"name": name, "in": "query", "example": []any{value, "$literal"}}
		data, err := json.Marshal(object{"openapi": "3.1.1", "servers": []any{object{"url": "https://example.test"}}, "paths": object{"/pets/{id}": object{"post": object{
			"parameters":  []any{parameter, object{"name": "id", "in": "path", "example": value}},
			"requestBody": object{"content": object{"application/json": object{"example": object{"value": value}}}},
		}}}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := collection.MarshalRequest(result.Requests[0])
		if err != nil {
			t.Fatal(err)
		}
		back, err := collection.ParseRequest(encoded, result.Requests[0].File)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := model.Resolve(back, model.MapLookup(map[string]string{"literal": "wrong", "id": "wrong", "query": "wrong"}))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(resolved.URL)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.EscapedPath() != "/pets/"+url.PathEscape(value) {
			t.Fatalf("path changed: %q => %q", value, parsed.EscapedPath())
		}
		if got := parsed.Query()[name]; !reflect.DeepEqual(got, []string{value, "$literal"}) {
			t.Fatalf("query changed: %q => %q", value, got)
		}
		var body map[string]string
		if err := json.Unmarshal([]byte(resolved.Body.Raw), &body); err != nil {
			t.Fatal(err)
		}
		if body["value"] != value {
			t.Fatalf("body changed: %q => %q", value, body["value"])
		}
	})
}

func TestProbeBoundedAdversarialDocuments(t *testing.T) {
	cases := []string{
		"openapi: 3.1.1\npaths: {}\nx: &x {child: *x}\n",
		`{"openapi":"3.1.1","paths":{"/x":{"get":{"parameters":[{"$ref":"#/components/parameters/a"}]}}},"components":{"parameters":{"a":{"$ref":"#/components/parameters/b"},"b":{"$ref":"#/components/parameters/a"}}}}`,
	}
	var aliases strings.Builder
	aliases.WriteString("openapi: 3.1.1\npaths: {}\nx0: &x0 [a,a,a,a,a,a,a,a,a,a]\n")
	for i := 1; i < 12; i++ {
		fmt.Fprintf(&aliases, "x%d: &x%d [%s]\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*x%d,", i-1), 10), ","))
	}
	cases = append(cases, aliases.String())
	for _, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Fatalf("accepted recursive/amplifying input: %.100s", data)
		}
	}
}

func TestProbeSchemaBudgetDoesNotHideLaterDefaults(t *testing.T) {
	data := []byte(`{"openapi":"3.1.1","paths":{"/a":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}}}},"/z":{"post":{"requestBody":{"content":{"application/json":{"schema":{"default":"keep"}}}}}}},"components":{"schemas":{"Node":{"type":"object","properties":{"left":{"$ref":"#/components/schemas/Node"},"right":{"$ref":"#/components/schemas/Node"}}}}}}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[1].Body.Raw != `"keep"` {
		t.Fatalf("earlier operation suppressed later explicit default: %q", result.Requests[1].Body.Raw)
	}
}

func persistedProbeRequest(t *testing.T, req model.Request) model.Request {
	t.Helper()
	encoded, err := collection.MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "request.posting.yaml")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	back, err := collection.ParseRequest(saved, req.File)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func TestProbeReferencedExampleOutputBound(t *testing.T) {
	paths := object{}
	for i := 0; i < 9; i++ {
		paths[fmt.Sprintf("/%d", i)] = object{"post": object{"requestBody": object{"$ref": "#/components/requestBodies/Shared"}}}
	}
	doc := object{"openapi": "3.1.1", "paths": paths, "components": object{"requestBodies": object{"Shared": object{"content": object{"application/json": object{"example": strings.Repeat("x", 4<<20)}}}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "materialized") {
		t.Fatalf("expected bounded-output error, got %v", err)
	}
}

func TestProbeSingleSchemaOutputBound(t *testing.T) {
	properties := object{}
	for i := 0; i < 40; i++ {
		properties[fmt.Sprintf("field%d", i)] = object{"$ref": "#/components/schemas/Shared"}
	}
	doc := object{"openapi": "3.1.1", "paths": object{"/x": object{"post": object{"requestBody": object{"content": object{"application/json": object{"schema": object{"type": "object", "properties": properties}}}}}}}, "components": object{"schemas": object{"Shared": object{"default": strings.Repeat("x", 1<<20)}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "materialized") {
		t.Fatalf("expected bounded-output error, got %v", err)
	}
}

func TestProbeLargeJSONStaysCompact(t *testing.T) {
	value := any(strings.Repeat("x", 200<<10))
	for i := 0; i < 40; i++ {
		value = []any{value}
	}
	doc := object{"openapi": "3.1.1", "paths": object{"/x": object{"post": object{"requestBody": object{"content": object{"application/json": object{"example": value}}}}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests[0].Body.Raw) > 201<<10 {
		t.Fatal("large JSON gained indentation")
	}
}
