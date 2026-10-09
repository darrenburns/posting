package postman

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/darrenburns/posting/v3/internal/env"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/importing"
	"github.com/darrenburns/posting/v3/internal/model"
)

func persistedProbe(t *testing.T, data []byte) (model.Request, map[string]string) {
	t.Helper()
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	written, err := importing.Write(result, dir)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join(dir, written.Files[0]))
	if err != nil {
		t.Fatal(err)
	}
	req, err := collection.ParseRequest(encoded, written.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, e := range written.Environments {
		raw, err := os.ReadFile(filepath.Join(dir, e.File))
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range env.Parse(string(raw), func(string) (string, bool) { return "HOST_MUST_NOT_LEAK", true }) {
			values[v.Name] = v.Value
		}
	}
	return req, values
}

func TestProbeAPIKeyReplacesExistingValues(t *testing.T) {
	for _, target := range []string{"header", "query"} {
		t.Run(target, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values := r.Header.Values("X-Key")
				if target == "query" {
					values = r.URL.Query()["X-Key"]
				}
				_ = json.NewEncoder(w).Encode(values)
			}))
			defer server.Close()
			data := wrap(fmt.Sprintf(`{"url":%q,"header":[{"key":"x-key","value":"old"},{"key":"X-Key","value":"older"}],"auth":{"type":"apikey","apikey":[{"key":"key","value":"X-Key"},{"key":"value","value":"replacement"},{"key":"in","value":%q}]}}`, server.URL+"?X-Key=old&X-Key=older", target))
			req, vars := persistedProbe(t, data)
			resp, err := client.NewHTTP("probe", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req, Variables: vars})
			if err != nil {
				t.Fatal(err)
			}
			if string(resp.Body) != "[\"replacement\"]\n" {
				t.Fatalf("API key must replace existing %s entries; got %s", target, resp.Body)
			}
		})
	}
}

func TestProbePathPercentEscapes(t *testing.T) {
	req, vars := persistedProbe(t, wrap(`{"url":{"protocol":"https","host":"example.com","path":[":id.json"],"variable":[{"key":"id","value":"a%20b"}]}}`))
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "https://example.com/a%20b.json" {
		t.Fatalf("path percent escape encoded twice: %s", resolved.URL)
	}
}

func TestProbeArrayPathRetainsLeadingEmptySegment(t *testing.T) {
	result, err := Parse(wrap(`{"url":{"protocol":"https","host":"example.com","path":["","foo"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].URL != "https://example.com//foo" {
		t.Fatalf("significant empty path segment dropped: %s", result.Requests[0].URL)
	}
}

func TestProbeNullFolderItemsRejected(t *testing.T) {
	_, err := Parse([]byte(`{"info":{},"item":[{"name":"valid","request":"https://example.com"},null]}`))
	if err == nil {
		t.Fatal("null item must be rejected")
	}
}

func TestProbeMalformedURLPartsRejected(t *testing.T) {
	for _, request := range []string{`{"url":{"host":[null]}}`, `{"url":{"host":["example",null]}}`, `{"url":{"host":"example.com","path":[null]}}`} {
		if _, err := Parse(wrap(request)); err == nil {
			t.Errorf("null URL part silently accepted: %s", request)
		}
	}
}

func TestProbeDuplicateAndDisabledVariables(t *testing.T) {
	data := []byte(`{"info":{},"variable":[{"key":"X","value":"old"},{"key":"X","value":"new"},{"key":"X","value":"disabled","disabled":true}],"item":[{"name":"local","variable":[{"key":"X","value":"local"},{"key":"X","value":"disabled","disabled":true}],"request":"https://example.com/{{X}}"},{"name":"sibling","request":"https://example.com/{{X}}"}]}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Variables) != 1 || result.Variables[0].Value != "new" || result.Requests[0].URL != "https://example.com/local" || result.Requests[1].URL != "https://example.com/${X}" {
		t.Fatalf("disabled/duplicate precedence: %#v", result)
	}
}

func TestProbeLiteralDollarStructuredMutations(t *testing.T) {
	values := []string{"$HOME", "${HOME}", "$$HOME", "a\"b\\c\n$HOME", "{{ROOT}}/$HOME", "雪/$x"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"info": map[string]string{"name": "Mutation"}, "variable": []any{map[string]string{"key": "ROOT", "value": "root"}}, "item": []any{map[string]any{"name": "One", "variable": []any{map[string]string{"key": "LOCAL", "value": value}}, "request": map[string]any{"url": "https://example.com", "header": []any{map[string]string{"key": "X-Test", "value": "{{LOCAL}}"}}, "body": map[string]string{"mode": "raw", "raw": "{{LOCAL}}"}}}}})
			req, vars := persistedProbe(t, data)
			vars["HOME"] = "unwanted"
			vars["x"] = "unwanted"
			resolved, err := model.Resolve(req, model.MapLookup(vars))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.ReplaceAll(value, "{{ROOT}}", "root")
			if resolved.Body.Raw != want || resolved.Headers[0].Value != want {
				t.Fatalf("literal mutated: got %#v want %q", resolved, want)
			}
		})
	}
}

func TestProbeInheritedAliasUsesLocalOverride(t *testing.T) {
	req, vars := persistedProbe(t, []byte(`{"info":{},"variable":[{"key":"BASE","value":"https://root.example"},{"key":"URL","value":"{{BASE}}/api"}],"item":[{"name":"Scoped","variable":[{"key":"BASE","value":"https://local.example"}],"request":"{{URL}}"}]}`))
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "https://local.example/api" {
		t.Fatalf("inherited alias ignored narrower variable scope: %s", resolved.URL)
	}
}

func TestProbePathSuffixWithEmptyExtension(t *testing.T) {
	req, vars := persistedProbe(t, wrap(`{"url":{"host":"example.com","path":[":id."],"variable":[{"key":"id","value":"42"}]}}`))
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "http://example.com/42." {
		t.Fatalf("path suffix lost: %s", resolved.URL)
	}
}

func TestProbeAPIKeyEmptyAndUnknownPlacement(t *testing.T) {
	result, err := Parse(wrap(`{"url":"https://example.com","auth":{"type":"apikey","apikey":{"key":"","value":""}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests[0].Headers) != 0 {
		t.Fatal("empty API key helper should not create a header")
	}
	result, err = Parse(wrap(`{"url":"https://example.com","auth":{"type":"apikey","apikey":{"key":"X-Key","value":"value","in":"surprise"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests[0].Headers) != 1 {
		t.Fatal("unknown API key target should default to header, as Postman runtime does")
	}
}

func TestProbeTotalExpansionBudget(t *testing.T) {
	// Each request is below the single-expansion limit, but inherited source
	// values would otherwise be duplicated indefinitely across requests.
	items := make([]any, 130)
	for i := range items {
		items[i] = map[string]any{"name": fmt.Sprintf("R%d", i), "request": map[string]any{"url": "https://example.com", "body": map[string]string{"mode": "raw", "raw": "{{PAYLOAD}}"}}}
	}
	data, _ := json.Marshal(map[string]any{"info": map[string]string{}, "item": []any{map[string]any{"name": "Folder", "variable": []any{map[string]string{"key": "PAYLOAD", "value": strings.Repeat("x", 256<<10)}}, "item": items}}})
	if _, err := Parse(data); err == nil {
		t.Fatal("total variable expansion should reject amplification over 32 MiB")
	}
}

// Structured fuzzing keeps every mutation a valid collection, reaching scope,
// query decoding, escaping and persistence that raw-byte fuzzing seldom reaches.
func FuzzProbeVariableQueryRoundtrip(f *testing.F) {
	for _, s := range []string{"plain", "$HOME", "${HOME}", "a+b&c%2F雪", "line\nquote\"\\", "", "\r\n"} {
		f.Add(s, true)
	}
	f.Fuzz(func(t *testing.T, value string, disabled bool) {
		if len(value) > 4096 {
			t.Skip()
		}
		value = string([]rune(value))
		value = strings.ReplaceAll(value, "{{", "{ {")
		data, _ := json.Marshal(map[string]any{
			"info":     map[string]string{"name": "Structured fuzz"},
			"variable": []any{map[string]string{"key": "ROOT", "value": value}},
			"item": []any{map[string]any{
				"name": "One", "variable": []any{map[string]any{"key": "LOCAL", "value": value, "disabled": false}, map[string]any{"key": "LOCAL", "value": "ignored", "disabled": true}},
				"request": map[string]any{
					"url":    map[string]any{"host": "example.com", "path": []string{"test"}, "query": []any{map[string]any{"key": "q", "value": url.QueryEscape(value), "disabled": disabled}}},
					"header": []any{map[string]string{"key": "X-Test", "value": "{{LOCAL}}"}},
					"body":   map[string]string{"mode": "raw", "raw": "{{ROOT}}"},
				},
			}},
		})
		req, vars := persistedProbe(t, data)
		resolved, err := model.Resolve(req, model.MapLookup(vars))
		if err != nil {
			t.Fatal(err)
		}
		expectedURL := "http://example.com/test"
		if !disabled {
			expectedURL += "?q=" + url.QueryEscape(value)
		}
		if resolved.URL != expectedURL || resolved.Body.Raw != value || resolved.Headers[0].Value != value {
			t.Fatalf("semantic divergence: URL=%q body=%q header=%q; want URL=%q value=%q", resolved.URL, resolved.Body.Raw, resolved.Headers[0].Value, expectedURL, value)
		}
		a, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Parse(data)
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("nondeterministic conversion")
		}
	})
}

func TestProbeAPIKeyQueryEncodedValues(t *testing.T) {
	req, vars := persistedProbe(t, wrap(`{"url":"https://example.com","auth":{"type":"apikey","apikey":{"key":"key","value":"a%2Fb+c","in":"query"}}}`))
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "https://example.com?key=a%2Fb+c" {
		t.Fatalf("API key encoding changed: %s", resolved.URL)
	}
}

func TestProbeReservedVariableNamesStayLiteralAndWarn(t *testing.T) {
	result, err := Parse(wrap(`{"url":"https://example.com/{{#}}?q={{&}}"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].URL != "https://example.com/{{#}}" || len(result.Requests[0].Query) != 1 || result.Requests[0].Query[0].Value != "{{&}}" || len(result.Warnings) < 2 {
		t.Fatalf("unsupported references silently split: %#v", result)
	}
}

func TestProbeEncodedQueryVariablesWarn(t *testing.T) {
	data := []byte(`{"info":{},"variable":[{"key":"VALUE","value":"a%2Fb+c"}],"item":[{"name":"One","request":{"url":"https://example.com?q={{VALUE}}"}}]}`)
	result, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "encoded query variable") {
		t.Fatal("encoded variable values change under Posting encoding; warn rather than silently changing semantics")
	}
}

func TestProbePercentEncodedTemplatesRemainLiteral(t *testing.T) {
	data := []byte(`{"info":{},"variable":[{"key":"ROOT","value":"wrong"}],"item":[{"name":"One","request":{"url":{"host":"example.com","path":[":id"],"query":[{"key":"encoded","value":"%7B%7BROOT%7D%7D"},{"key":"dollar","value":"%24ROOT"}],"variable":[{"key":"id","value":"%7B%7BROOT%7D%7D"}]}}}]}`)
	req, vars := persistedProbe(t, data)
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	want := "http://example.com/%7B%7BROOT%7D%7D?encoded=%7B%7BROOT%7D%7D&dollar=%24ROOT"
	if resolved.URL != want {
		t.Fatalf("encoded literal became variable: got %s want %s", resolved.URL, want)
	}
}

func TestProbeAPIKeyEncodedNameOverrides(t *testing.T) {
	req, vars := persistedProbe(t, wrap(`{"url":"https://example.com?X%20Key=old","auth":{"type":"apikey","apikey":{"key":"X%20Key","value":"new","in":"query"}}}`))
	resolved, err := model.Resolve(req, model.MapLookup(vars))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "https://example.com?X+Key=new" {
		t.Fatalf("encoded auth key failed to replace query field: %s", resolved.URL)
	}
}
