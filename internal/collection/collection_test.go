package collection

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

// The Posting 2 test collection lives in the repository's Python test suite.
const sampleDir = "../../tests/sample-collections"

func TestLoadPosting2Collection(t *testing.T) {
	root, problems := Dir{Root: sampleDir}.Load()
	if len(problems) > 0 {
		t.Fatalf("load problems: %v", problems)
	}
	if root.Name != "sample-collections" {
		t.Errorf("root name = %q", root.Name)
	}
	byFile := map[string]model.Request{}
	root.Walk(func(_ *model.Collection, r model.Request) { byFile[r.File] = r })
	if len(byFile) != 17 {
		t.Errorf("loaded %d requests, want 17", len(byFile))
	}
	// The scripts folder has no requests, so it isn't shown.
	for _, child := range root.Children {
		if child.Name == "scripts" {
			t.Error("folders without requests should be left out")
		}
	}

	echo := byFile["echo-post-01.posting.yaml"]
	if echo.Method != model.MethodPost || echo.Body.Type != model.BodyRaw || echo.Body.ContentType != "application/json" {
		t.Errorf("echo post body = %+v", echo.Body)
	}
	if !strings.Contains(echo.Body.Raw, `"Hello, world!"`) {
		t.Errorf("echo post raw body = %q", echo.Body.Raw)
	}
	if echo.Auth != (model.Auth{Type: model.AuthBasic, Username: "darren"}) {
		t.Errorf("auth = %+v", echo.Auth)
	}
	if echo.Options.TimeoutSeconds != 0.2 || !echo.Options.FollowRedirects {
		t.Errorf("options = %+v", echo.Options)
	}
	wantQuery := []model.KeyValue{{Name: "key1", Value: "value1", Enabled: true}, {Name: "another-key", Value: "another-value", Enabled: true}, {Name: "number", Value: "123", Enabled: true}}
	if !reflect.DeepEqual(echo.Query, wantQuery) {
		t.Errorf("query = %+v", echo.Query)
	}

	create := byFile["jsonplaceholder/users/create.posting.yaml"]
	if create.Scripts.OnRequest != "scripts/my_script.py" || create.Options.FollowRedirects {
		t.Errorf("create user = %+v", create)
	}
	users := findChild(findChild(root, "jsonplaceholder"), "users")
	if users == nil || users.Path != "jsonplaceholder/users" || len(users.Requests) != 5 {
		t.Fatalf("users folder = %+v", users)
	}
	// GET, POST, PUT/PATCH, DELETE order, as in Posting 2.
	if users.Requests[0].Method != model.MethodGet || users.Requests[len(users.Requests)-1].Method != model.MethodDelete {
		t.Errorf("requests aren't sorted by method: %v", users.Requests)
	}
}

func TestRoundTripPosting2Files(t *testing.T) {
	root, _ := Dir{Root: sampleDir}.Load()
	root.Walk(func(_ *model.Collection, r model.Request) {
		data, err := MarshalRequest(r)
		if err != nil {
			t.Fatalf("%s: %v", r.File, err)
		}
		back, err := ParseRequest(data, r.File)
		if err != nil {
			t.Fatalf("%s: re-parse: %v\n%s", r.File, err, data)
		}
		if !reflect.DeepEqual(back, r) {
			t.Errorf("%s changed in a round trip\nbefore: %+v\nafter:  %+v\n%s", r.File, r, back, data)
		}
	})
}

func TestMarshalOmitsDefaults(t *testing.T) {
	req := model.NewRequest()
	req.Name = "Ping"
	req.URL = "https://example.com"
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "name: Ping\nurl: https://example.com\n"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}

	req.Method = model.MethodPost
	req.Description = "Two\nlines "
	req.Headers = []model.KeyValue{{Name: "X-Off", Value: "1", Enabled: false}}
	req.Body = model.Body{Type: model.BodyRaw, Raw: "{\n  \"a\": 1\n}", ContentType: "application/json"}
	req.Options.VerifySSL = false
	req.Options.TimeoutSeconds = 30
	data, _ = MarshalRequest(req)
	want := `name: Ping
description: "Two\nlines "
method: POST
url: https://example.com
body:
  content: |-
    {
      "a": 1
    }
  content_type: application/json
headers:
  - name: X-Off
    value: "1"
    enabled: false
options:
  verify_ssl: false
  timeout: 30
`
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
}

func TestDirSaveAndDelete(t *testing.T) {
	dir := Dir{Root: t.TempDir()}
	req := model.NewRequest()
	req.Name = "Nested"
	req.File = "a/b/nested.posting.yaml"
	if err := dir.Save(req); err != nil {
		t.Fatal(err)
	}
	root, problems := dir.Load()
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	b := findChild(findChild(root, "a"), "b")
	if b == nil || len(b.Requests) != 1 || b.Requests[0].Name != "Nested" {
		t.Fatalf("saved request not loaded: %+v", root)
	}
	if err := dir.Delete(req.File); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir.Root, "a/b/nested.posting.yaml")); !os.IsNotExist(err) {
		t.Fatal("file still exists after delete")
	}
	for _, bad := range []string{"../escape.posting.yaml", "/abs.posting.yaml", "a/../../x.posting.yaml", "notes.txt"} {
		req.File = bad
		if err := dir.Save(req); err == nil {
			t.Errorf("saving to %q should fail", bad)
		}
	}
}

func TestLoadReportsBadFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.posting.yaml"), []byte("name: ok\nurl: https://x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.posting.yaml"), []byte("name: [unclosed\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "method.posting.yaml"), []byte("method: FETCH\n"), 0o644)
	root, problems := Dir{Root: dir}.Load()
	if len(root.Requests) != 1 || len(problems) != 2 {
		t.Fatalf("requests = %d, problems = %v", len(root.Requests), problems)
	}
}

func findChild(c *model.Collection, name string) *model.Collection {
	if c == nil {
		return nil
	}
	for _, child := range c.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func TestMarshalKeepsQueryOnlyInParams(t *testing.T) {
	req := model.NewRequest()
	req.URL = "https://x.test/items?a=1&b=2#top"
	req.Query = []model.KeyValue{{Name: "a", Value: "1", Enabled: true}, {Name: "b", Value: "2", Enabled: true}, {Name: "c", Value: "3", Enabled: false}}
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	back, _ := ParseRequest(data, "")
	if back.URL != "https://x.test/items#top" || len(back.Query) != 3 {
		t.Fatalf("url = %q, query = %+v\n%s", back.URL, back.Query, data)
	}

	// A query with no parameter rows is left in the URL.
	req.Query = nil
	data, _ = MarshalRequest(req)
	if back, _ := ParseRequest(data, ""); back.URL != req.URL {
		t.Fatalf("url = %q", back.URL)
	}
}

func TestEmptyFormRoundTrip(t *testing.T) {
	req := model.NewRequest()
	req.Body = model.Body{Type: model.BodyForm, ContentType: "application/x-www-form-urlencoded"}
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseRequest(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if back.Body.Type != model.BodyForm {
		t.Fatalf("empty form changed type to %q after save/reload; YAML: %s", back.Body.Type, data)
	}
}

func TestDirSavePreservesFragmentWithDisabledQuery(t *testing.T) {
	for _, rawURL := range []string{
		"https://api.test/search#results?view=compact",
		"${BASE_URL}/search#results?view=compact",
		"https://api.test/search#?view=compact",
		"https://api.test/search#",
		"https://api.test/search#results",
		"https://api.test/search",
	} {
		t.Run(rawURL, func(t *testing.T) {
			dir := Dir{Root: t.TempDir()}
			req := model.NewRequest()
			req.File = "search.posting.yaml"
			// Disabling the last enabled query row in the editor leaves the
			// URL without a query but retains the disabled row in the table.
			req.URL = rawURL
			req.Query = []model.KeyValue{{Name: "q", Value: "saved", Enabled: false}}
			for save := 1; save <= 2; save++ {
				if err := dir.Save(req); err != nil {
					t.Fatal(err)
				}
				root, problems := dir.Load()
				if len(problems) != 0 || len(root.Requests) != 1 {
					t.Fatalf("saved request could not be loaded: %+v, %v", root, problems)
				}
				got := root.Requests[0]
				if got.URL != rawURL {
					t.Fatalf("save %d changed URL: got %q, want %q", save, got.URL, rawURL)
				}
				if !reflect.DeepEqual(got.Query, req.Query) {
					t.Fatalf("save %d changed disabled query rows: got %+v, want %+v", save, got.Query, req.Query)
				}
				req = got
			}
		})
	}
}
