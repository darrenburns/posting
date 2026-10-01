package bruno

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestProbeDictionaryDelimiterContexts(t *testing.T) {
	for _, tc := range []struct{ name, block, want string }{
		{"quoted annotation apostrophes", "headers {\n  @description(\"'''\")\n  X-Test: yes\n}\n", "yes"},
		{"Unicode multiline slice", "headers {\n  X-Test: '''éabcvalue'''\n}\n", "value"},
		{"astral multiline slice", "headers {\n  X-Test: '''😀abvalue'''\n}\n", "value"},
		{"literal apostrophes", "headers {\n  X-Test: a'''b\n}\n", "a'''b"},
		{"quoted apostrophe key", "headers {\n  \"X-'''\": yes\n}\n", "yes"},
		{"inline multiline", "headers { X-Test: '''\n    before\n}\n    after\n  '''\n}\n", "before\n\nafter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse([]byte("get {\n  url: https://example.test\n}\n" + tc.block))
			if err != nil {
				t.Fatal(err)
			}
			if got := r.Requests[0].Headers[0].Value; got != tc.want {
				t.Fatalf("value = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestProbeDisabledQuotedKey(t *testing.T) {
	r, err := Parse([]byte("get {\n url: https://example.test\n}\nparams:query {\n ~\"~key\": value\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	q := r.Requests[0].Query[0]
	if q.Name != "~key" || q.Enabled {
		t.Fatalf("row = %+v; want disabled ~key", q)
	}
}

func TestProbeInlineTextWhitespace(t *testing.T) {
	r, err := Parse([]byte("post {\n url: https://example.test\n body: text\n}\nbody:text {  payload  \n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Requests[0].Body.Raw; got != "payload  " {
		t.Fatalf("body = %q", got)
	}
}

// Check the full import -> Posting file -> load -> real HTTP request path. Bruno
// chooses JSON escaping from the effective Content-Type, including inherited or
// interpolated headers, independently of the selected editor body mode.
func TestProbeBodyContentTypeOnWire(t *testing.T) {
	for _, tc := range []struct{ name, mode, header, want string }{
		{"text with JSON header", "text", "Content-Type: application/json", `{"value":"a\"b"}`},
		{"JSON with text header", "json", "Content-Type: text/plain", `{"value":"a"b"}`},
		{"interpolated header", "text", "Content-Type: {{ctype}}", `{"value":"a\"b"}`},
		{"disabled header", "json", "~Content-Type: text/plain", `{"value":"a\"b"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { b, _ := io.ReadAll(r.Body); got = string(b) }))
			defer server.Close()
			input := fmt.Sprintf("post {\n url: %s\n body: %s\n}\nheaders {\n %s\n}\nvars:pre-request {\n name: a\"b\n ctype: application/json\n}\nbody:%s {\n  {\"value\":\"{{name}}\"}\n}\n", server.URL, tc.mode, tc.header, tc.mode)
			imported, err := Parse([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			dir := collection.Dir{Root: t.TempDir()}
			if err := dir.Save(imported.Requests[0]); err != nil {
				t.Fatal(err)
			}
			loaded, errs := dir.Load()
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(loaded.Requests) != 1 {
				t.Fatal("missing request")
			}
			_, err = client.NewHTTP("probe", client.TLSSettings{}).Send(context.Background(), client.Call{Request: loaded.Requests[0]})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("wire body = %q, want %q", got, tc.want)
			}
		})
	}
}

func FuzzProbeJSONSubstitution(f *testing.F) {
	for _, value := range []string{`a"b`, `\`, `$dollar`, `${LITERAL}`, "apostrophe'''", "日本語", `abc\"def`} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 256 || !utf8.ValidString(value) || strings.ContainsAny(value, "\n\r\x00") || strings.Contains(value, "{{") || strings.HasPrefix(value, "'''") || value == "[" || strings.TrimSpace(value) != value {
			t.Skip()
		}
		input := "post {\n url: https://example.test\n body: json\n}\nvars:pre-request {\n value: " + value + "\n}\nbody:json {\n  {\"value\":\"{{value}}\"}\n}\n"
		imported, err := Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		data, err := collection.MarshalRequest(imported.Requests[0])
		if err != nil {
			t.Fatal(err)
		}
		back, err := collection.ParseRequest(data, "")
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := model.Resolve(back, model.MapLookup(nil))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]string
		if err := json.Unmarshal([]byte(resolved.Body.Raw), &got); err != nil {
			t.Fatalf("invalid JSON %q: %v", resolved.Body.Raw, err)
		}
		if got["value"] != value {
			t.Fatalf("resolved JSON value=%q, want=%q", got["value"], value)
		}
	})
}

func TestProbeAggregateExpansionBound(t *testing.T) {
	// Every individual value is below 16 MiB, but their combined materialization
	// must not grow without a request-wide limit.
	var input strings.Builder
	input.WriteString("get {\n url: https://example.test\n}\nvars:pre-request {\n large: ")
	input.WriteString(strings.Repeat("x", 1<<20))
	input.WriteString("\n}\nheaders {\n")
	for i := 0; i < 17; i++ {
		fmt.Fprintf(&input, " X-%d: {{large}}\n", i)
	}
	input.WriteString("}\n")
	if _, err := Parse([]byte(input.String())); err == nil {
		t.Fatal("accepted more than 16 MiB of aggregate variable expansion")
	}
}

func TestProbeCollectionFilesystemBounds(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bruno.json"), []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, strings.Repeat("d/", 65)), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "64 folder") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("metadata symlink", func(t *testing.T) {
		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "bruno.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatal("accepted symlink metadata")
		}
	})
	t.Run("nested directory symlink", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bruno.json"), []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(dir, filepath.Join(dir, "cycle")); err != nil {
			t.Fatal(err)
		}
		got, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Requests) != 0 || !strings.Contains(strings.Join(got.Warnings, " "), "symlink") {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("file size", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.bru")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(maxFileSize + 1); err != nil {
			t.Fatal(err)
		}
		f.Close()
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "16 MiB") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("row count", func(t *testing.T) {
		input := "get {\n url: https://example.test\n}\nheaders {\n" + strings.Repeat(" x: y\n", 10001) + "}\n"
		if _, err := Parse([]byte(input)); err == nil || !strings.Contains(err.Error(), "10000 rows") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestProbeCollectionAggregateExpansionBound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bruno.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	vars := "vars:pre-request {\n large: " + strings.Repeat("x", 1<<20) + "\n}\n"
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "folder", "folder.bru"), []byte(vars), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 129; i++ {
		input := "post {\n url: https://example.test\n body: text\n}\nbody:text {\n  {{large}}\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "folder", fmt.Sprintf("%03d.bru", i)), []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("accepted more than 128 MiB of materialized collection data")
	}
}

func TestProbeScopedFormRequestOnWire(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/item/a b" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query()["q"]; len(got) != 2 || got[0] != "one" || got[1] != "two" {
			t.Errorf("query=%v", got)
		}
		if r.Header.Get("X-Scope") != "folder" || r.Header.Get("Authorization") != "Bearer token$literal" {
			t.Errorf("scope/auth not preserved")
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("value") != "a&b+$literal" || r.PostForm.Has("off") {
			t.Errorf("form=%v", r.PostForm)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	files := map[string]string{
		"bruno.json":        `{}`,
		"collection.bru":    "auth {\n mode: bearer\n}\nauth:bearer {\n token: {{token}}\n}\nvars:pre-request {\n base: " + server.URL + "\n token: token$literal\n}\nheaders {\n X-Scope: root\n}\n",
		"folder/folder.bru": "headers {\n X-Scope: folder\n}\n",
		"folder/one.bru":    "post {\n url: {{base}}/item/:id?q=one&q=two\n body: form-urlencoded\n auth: inherit\n}\nheaders {\n ~X-Scope: disabled\n}\nparams:path {\n id: a b\n}\nbody:form-urlencoded {\n value: a&b+$literal\n ~off: no\n}\n",
	}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	imported, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	target := collection.Dir{Root: t.TempDir()}
	if _, err := importing.Write(imported, target.Root); err != nil {
		t.Fatal(err)
	}
	loaded, errs := target.Load()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	environment, err := env.Load(env.Stack(target.Root, env.BaseName))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.NewHTTP("probe", client.TLSSettings{}).Send(context.Background(), client.Call{Request: loaded.Children[0].Requests[0], Variables: model.Values(environment.Variables)})
	if err != nil {
		t.Fatal(err)
	}
}
