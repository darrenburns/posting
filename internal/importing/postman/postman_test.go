package postman

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

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

func TestCollectionFixtureRoundtrip(t *testing.T) {
	data, err := os.ReadFile("testdata/collection.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Example API" || len(result.Requests) != 4 || len(result.Variables) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	create := result.Requests[0]
	if create.File != "Users/Create.posting.yaml" || create.URL != "https://example.com/users/:id.json" || create.Auth.Token != "local" || create.Description != "Create a user" {
		t.Fatalf("unexpected create: %#v", create)
	}
	if create.Query[0].Value != "a+b c" || create.Query[2].Enabled || create.Headers[1].Enabled {
		t.Fatalf("disabled/encoded fields not preserved: %#v", create)
	}
	if create.Body.ContentType != "application/json" || create.Body.Raw != `{"price":"$$10","token":"local"}` {
		t.Fatalf("body: %#v", create.Body)
	}
	if result.Requests[1].Auth.Type != model.AuthNone || result.Requests[1].Body.Type != model.BodyNone {
		t.Fatalf("noauth/disabled body: %#v", result.Requests[1])
	}
	if result.Requests[2].Auth.Token != "${TOKEN}" {
		t.Fatal("folder variable leaked to sibling")
	}
	for _, req := range result.Requests {
		b, err := collection.MarshalRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := collection.ParseRequest(b, req.File)
		if err != nil {
			t.Fatal(err)
		}
		before, err := model.Resolve(req, model.MapLookup(map[string]string{"BASE": "https://example.com", "TOKEN": "global"}))
		if err != nil {
			t.Fatal(err)
		}
		after, err := model.Resolve(loaded, model.MapLookup(map[string]string{"BASE": "https://example.com", "TOKEN": "global"}))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("roundtrip changed request:\n%#v\n%#v", before, after)
		}
	}
}

func TestImportedRequestOnWire(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		user, pass, _ := r.BasicAuth()
		_ = json.NewEncoder(w).Encode(map[string]string{"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "body": string(data), "user": user, "password": pass, "literal": r.Header.Get("X-Literal"), "off": r.Header.Get("X-Off"), "type": r.Header.Get("Content-Type")})
	}))
	defer server.Close()
	data := []byte(fmt.Sprintf(`{"info":{"name":"Wire","schema":"https://schema.getpostman.com/json/collection/v2.0.0/collection.json"},"auth":{"type":"basic","basic":{"username":"user","password":"$PASSWORD"}},"item":[{"name":"Form","request":{"method":"POST","url":%q,"header":"X-Literal: $HOME\r\nX-More: yes","body":{"mode":"urlencoded","urlencoded":[{"key":"q","value":"one"},{"key":"q","value":"two"},{"key":"off","value":"bad","disabled":true}]}}}]}`, server.URL+"/hello?q=a%2Bb%20c&q=two"))
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := collection.MarshalRequest(result.Requests[0])
	if err != nil {
		t.Fatal(err)
	}
	req, err := collection.ParseRequest(b, "")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.NewHTTP("test", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req, Variables: map[string]string{"HOME": "wrong", "PASSWORD": "wrong"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(resp.Body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"method": "POST", "path": "/hello", "query": "q=a%2Bb+c&q=two", "body": "q=one&q=two", "user": "user", "password": "$PASSWORD", "literal": "$HOME", "off": "", "type": "application/x-www-form-urlencoded"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func wrap(request string) []byte {
	return []byte(`{"info":{"name":"Test"},"item":[{"name":"One","request":` + request + `}]}`)
}
func TestURLRepresentations(t *testing.T) {
	for _, tt := range []struct {
		name, request, url string
		query              int
		warnings           bool
	}{
		{"string request", `"https://example.com?a=1"`, "https://example.com", 1, false},
		{"raw only", `{"url":{"raw":"https://example.com?q=old","query":[]}}`, "https://example.com", 0, false},
		{"disabled query", `{"url":{"raw":"https://example.com?q=old","query":[{"key":"q","value":"new","disabled":true}]}}`, "https://example.com", 1, false},
		{"path objects", `{"url":{"protocol":"https","host":"example.com","port":"8443","path":[{"value":"a"},"b"],"hash":"fragment"}}`, "https://example.com:8443/a/b#fragment", 0, false},
		{"bare query", `{"url":"https://example.com?bare"}`, "https://example.com", 1, true},
		{"fragment question mark", `{"url":"https://example.com#fragment?notQuery"}`, "https://example.com#fragment?notQuery", 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(wrap(tt.request))
			if err != nil {
				t.Fatal(err)
			}
			req := result.Requests[0]
			if req.URL != tt.url || len(req.Query) != tt.query || (len(result.Warnings) > 0) != tt.warnings {
				t.Fatalf("%#v", result)
			}
		})
	}
}
func TestVariablesAuthAndWarnings(t *testing.T) {
	data := []byte(`{"info":{"name":"Scopes"},"variable":[{"key":"A","value":"https://example.com"},{"key":"B","value":"{{A}}/api"},{"key":"C","value":"{{MISSING}}"},{"key":"bad-name","value":"x"}],"auth":{"type":"bearer","bearer":[{"key":"token","value":"root"}]},"event":[{}],"item":[{"name":"Folder","auth":{"type":"basic","basic":[{"key":"username","value":"{{LOCAL}}"},{"key":"password","value":"p"}]},"variable":[{"key":"LOCAL","value":"local"}],"item":[{"name":"One","request":{"url":"{{B}}/{{LOCAL}}","auth":null,"body":{"mode":"formdata"},"header":[{"key":"Dynamic","value":"{{$randomInt}} {{vault:secret}} {{bad-name}}"}]}}]},{"name":"Sibling","request":{"url":"{{A}}","auth":{"type":"apikey","apikey":[{"key":"key","value":"key"},{"key":"value","value":"secret"},{"key":"in","value":"query"}]}}}]}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Variables[1].Value != "${A}/api" {
		t.Fatalf("collection variables keep references: %#v", result.Variables)
	}
	if result.Requests[0].URL != "${B}/local" || result.Requests[0].Auth.Username != "local" {
		t.Fatalf("scoped auth: %#v", result.Requests[0])
	}
	if result.Requests[1].Query[0].Value != "secret" {
		t.Fatal("API key missing")
	}
	warnings := strings.Join(result.Warnings, "\n")
	for _, needle := range []string{"scripts", "formdata", "dynamic", "bad-name"} {
		if !strings.Contains(warnings, needle) {
			t.Fatalf("warning missing %q: %s", needle, warnings)
		}
	}
}
func TestInvalidCollections(t *testing.T) {
	for _, s := range []string{"", `null`, `[]`, `{}`, `{"info":{},"item":[]}`, `{"info":{},"item":[{}]}`, `{"info":{},"item":[{"request":{}}]}`, string(wrap(`{"url":7}`)), string(wrap(`{"url":"https://example.com","method":"PROPFIND"}`)), string(wrap(`{"url":"https://example.com","auth":{}}`)), string(wrap(`{"url":"https://example.com","header":42}`))} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("expected error for %s", s)
		}
	}
}
func TestVariableExpansionIsBounded(t *testing.T) {
	vars := []map[string]string{}
	for i := 0; i < 70; i++ {
		vars = append(vars, map[string]string{"key": fmt.Sprintf("V%d", i), "value": fmt.Sprintf("{{V%d}}{{V%d}}", i+1, i+1)})
	}
	data, _ := json.Marshal(map[string]any{"info": map[string]string{"name": "Expansion"}, "item": []any{map[string]any{"name": "One", "variable": vars, "request": map[string]string{"url": "https://example.com/{{V0}}"}}}})
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected expansion warning")
	}
}
func FuzzParse(f *testing.F) {
	fixture, _ := os.ReadFile("testdata/collection.json")
	f.Add(fixture)
	f.Add(wrap(`{"url":{"raw":"https://example.com?a=1","query":[{"key":"x","value":"%2B"}]}}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		result, err := Parse(data)
		if err != nil {
			return
		}
		again, err := Parse(data)
		if err != nil || !reflect.DeepEqual(result, again) {
			t.Fatal("non-deterministic parse")
		}
		for _, req := range result.Requests {
			encoded, err := collection.MarshalRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := collection.ParseRequest(encoded, req.File); err != nil {
				t.Fatalf("cannot reload imported request: %v", err)
			}
		}
	})
}
