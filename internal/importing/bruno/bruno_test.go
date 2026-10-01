package bruno

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

const sample = `meta {
  name: Send event
  type: http
}
post {
  url: {{base}}/events/:id?repeat=1&repeat=2#top
  body: json
  auth: basic
}
headers {
  "x:quoted": yes
  ~"disabled:key": retained
  X-Token: {{token}}
}
params:query {
  ~off: disabled
}
params:path {
  id: {{id}}
}
auth:basic {
  username: user
  password: $literal
}
vars:pre-request {
  base: https://example.test
  id: 42
}
body:json {
  {
    "name": "{{name}}",
    "value": "$literal"
  }
}
docs {
  Request documentation.
}
`

func TestParseAndRoundTrip(t *testing.T) {
	result, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	r := result.Requests[0]
	if r.URL != "https://example.test/events/:id#top" || r.Method != model.MethodPost || len(r.Query) != 3 || r.Query[0].Value != "1" || r.Query[1].Value != "2" || r.Query[2].Enabled {
		t.Fatalf("request: %+v", r)
	}
	if r.Headers[0].Name != "x:quoted" || r.Headers[1].Enabled || r.Headers[1].Name != "disabled:key" || r.Headers[2].Value != "${token}" {
		t.Fatalf("headers: %+v", r.Headers)
	}
	if r.Auth.Password != "$$literal" || r.PathParams[0].Value != "42" || !strings.Contains(r.Body.Raw, `"name": "${name}"`) {
		t.Fatalf("request: %+v", r)
	}
	data, err := collection.MarshalRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	back, err := collection.ParseRequest(data, r.File)
	if err != nil {
		t.Fatal(err)
	}
	if back.Body.Raw != r.Body.Raw || back.URL != r.URL || len(back.Query) != len(r.Query) || back.Auth != r.Auth {
		t.Fatalf("roundtrip changed request: %+v\n%s", back, data)
	}
}
func TestRawBodiesUseBruIndentation(t *testing.T) {
	input := "post {\n  url: https://example.test\n  body: text\n}\nbody:text {\n  }\n    deeper\n  trailing  \n\n}\n"
	result, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Requests[0].Body.Raw, "}\n  deeper\ntrailing  \n"; got != want {
		t.Fatalf("raw=%q want=%q", got, want)
	}
	data, err := collection.MarshalRequest(result.Requests[0])
	if err != nil {
		t.Fatal(err)
	}
	back, err := collection.ParseRequest(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if back.Body.Raw != result.Requests[0].Body.Raw {
		t.Fatalf("roundtrip: %q", back.Body.Raw)
	}
}
func TestFormMultilineAndQuotedKeys(t *testing.T) {
	input := `post {
  url: https://example.test
  body: form-urlencoded
}
body:form-urlencoded {
  "nested escaped \"quote\"": allowed
  ~off: no
  value: '''
    alpha
    beta
  '''
}
`
	result, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	rows := result.Requests[0].Body.Form
	if rows[0].Name != `nested escaped "quote"` || rows[1].Enabled || rows[2].Value != "alpha\nbeta" {
		t.Fatalf("form=%+v", rows)
	}
}
func TestCollectionInheritanceAndSiblingIsolation(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("bruno.json", `{"name":"Example","type":"collection"}`)
	write("collection.bru", "auth {\n  mode: basic\n}\nauth:basic {\n  username: {{who}}\n  password: root\n}\nheaders {\n  X-Test: root\n}\nvars:pre-request {\n  base: https://example.test\n  who: root\n}\n")
	write("A/folder.bru", "auth {\n  mode: none\n}\nvars:pre-request {\n  who: A\n}\nheaders {\n  x-test: A\n}\n")
	write("A/one.bru", "get {\n  url: {{base}}/{{who}}\n  auth: inherit\n}\n")
	write("B/two.bru", "get {\n  url: {{base}}/{{who}}\n  auth: inherit\n}\nheaders {\n  ~X-Test: disabled\n}\n")
	write("environments/Production.bru", "vars {\n  token: secret\n}\n")
	result, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests) != 2 || len(result.Environments) != 1 || result.Environments[0].Name != "Production" {
		t.Fatalf("%+v", result)
	}
	requests, _ := imported(t, result, "Production")
	a, b := requests["A/one.posting.yaml"], requests["B/two.posting.yaml"]
	if a.URL != "https://example.test/A" || b.URL != "https://example.test/root" || a.Auth.Type != model.AuthNone || b.Auth.Type != model.AuthBasic || b.Auth.Username != "root" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	if a.Headers[0].Value != "A" || b.Headers[0].Value != "root" || len(b.Headers) != 2 || b.Headers[1].Enabled {
		t.Fatalf("headers a=%+v b=%+v", a.Headers, b.Headers)
	}
}
func TestUnsupportedFeaturesWarn(t *testing.T) {
	input := `post {
  url: https://example.test
  body: multipart-form
  auth: oauth2
}
body:multipart-form {
  upload: @file(/etc/passwd)
}
script:pre-request {
  throw new Error("never execute");
}
assert {
  res.status: 200
}
`
	result, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(result.Warnings, " ")
	for _, want := range []string{"multipart-form", "oauth2", "script:pre-request", "assert"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("missing warning %s: %s", want, warnings)
		}
	}
	if result.Requests[0].Body.Type != model.BodyNone {
		t.Fatal("unsupported body imported")
	}
}
func TestReferencesAreBoundedAndDoNotExpandDollars(t *testing.T) {
	input := "get {\n  url: https://example.test/{{a}}/{{process.env.HOST}}/$literal\n}\nvars:pre-request {\n  a: {{b}}\n  b: {{a}}\n}\n"
	result, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].URL != "https://example.test/{{a}}/${HOST}/$$literal" || len(result.Warnings) == 0 {
		t.Fatalf("%+v", result)
	}
}
func TestInvalidInputs(t *testing.T) {
	for _, input := range []string{"", "get {\n url: x", "get { url: x }", "get {\n url: x\n}\nget {\n url: y\n}", "get {\n url: x\n}\nheaders {\n broken\n}", "get {\n url: x\n}\nsettings {\n timeout: NaN\n}"} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
}
func TestLoadRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.bru")
	if err := os.WriteFile(target, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.bru")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("accepted symlink input")
	}
	if err := os.WriteFile(filepath.Join(dir, "bruno.json"), []byte(`{"name":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("%+v", result)
	}
}
func FuzzParse(f *testing.F) {
	f.Add([]byte(sample))
	f.Add([]byte("get {\n url: https://example.test\n}\n"))
	f.Add([]byte("meta {\n type: grpc\n}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := Parse(data)
		if err != nil {
			return
		}
		for _, r := range result.Requests {
			encoded, err := collection.MarshalRequest(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := collection.ParseRequest(encoded, r.File); err != nil {
				t.Fatalf("produced invalid Posting file: %v\n%s", err, encoded)
			}
		}
	})
}

func TestJSONVariableEscaping(t *testing.T) {
	input := `post {
  url: https://example.test
  body: json
}
vars:pre-request {
  name: a"b
  count: 42
}
body:json {
  {"name":"{{name}}", "count":{{count}}}
}
`
	result, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Requests[0].Body.Raw; got != `{"name":"a\"b", "count":42}` {
		t.Fatalf("JSON body: %q", got)
	}
}
func TestInlineDictionaryAndLocalVariable(t *testing.T) {
	result, err := Parse([]byte("get { url: https://example.test/{{local}}\n}\nvars:pre-request {\n  @local: function(value)\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Requests[0].URL != "https://example.test/function(value)" {
		t.Fatal(result.Requests[0].URL)
	}
}
func TestExpansionBombRejected(t *testing.T) {
	var b strings.Builder
	b.WriteString("get {\n url: https://example.test/{{v0}}\n}\nvars:pre-request {\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, " v%d: {{v%d}}{{v%d}}\n", i, i+1, i+1)
	}
	b.WriteString(" v20: x\n}\n")
	if _, err := Parse([]byte(b.String())); err == nil {
		t.Fatal("accepted expansion bomb")
	}
}
