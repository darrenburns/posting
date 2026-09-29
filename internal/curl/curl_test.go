package curl

import (
	"reflect"
	"testing"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

func TestSplit(t *testing.T) {
	cases := map[string][]string{
		`curl -H 'A: b c' "x \"y\" \$z"`:     {"curl", "-H", "A: b c", `x "y" $z`},
		"curl \\\n  -X POST \\\n  url":       {"curl", "-X", "POST", "url"},
		`curl --data-raw $'{"a":"it\'s\n"}'`: {"curl", "--data-raw", "{\"a\":\"it's\n\"}"},
		`curl $'\x41\u00e9\101'`:             {"curl", "Aé" + "A"},
		`curl a\ b 'c'"d"e`:                  {"curl", "a b", "cde"},
		`curl ''`:                            {"curl", ""},
	}
	for input, want := range cases {
		got, err := Split(input)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, bad := range []string{`curl 'open`, `curl "open`, `curl $'open`} {
		if _, err := Split(bad); err == nil {
			t.Errorf("Split(%q) should fail", bad)
		}
	}
}

func TestParseBrowserCommand(t *testing.T) {
	// As copied from Chrome's network panel.
	command := `curl 'https://api.example.com/v1/items?page=2&q=a%20b' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer abc.def' \
  -H 'content-type: application/json' \
  -b 'session=1; theme=dark' \
  --data-raw $'{"name":"it\'s"}' \
  --compressed`
	req, err := Parse(command)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != model.MethodPost || req.URL != "https://api.example.com/v1/items?page=2&q=a%20b" {
		t.Errorf("method/url = %s %s", req.Method, req.URL)
	}
	wantQuery := []model.KeyValue{{Name: "page", Value: "2", Enabled: true}, {Name: "q", Value: "a b", Enabled: true}}
	if !reflect.DeepEqual(req.Query, wantQuery) {
		t.Errorf("query = %+v", req.Query)
	}
	if req.Auth != (model.Auth{Type: model.AuthBearer, Token: "abc.def"}) {
		t.Errorf("auth = %+v", req.Auth)
	}
	wantHeaders := []model.KeyValue{
		{Name: "accept", Value: "application/json", Enabled: true},
		{Name: "content-type", Value: "application/json", Enabled: true},
		{Name: "Cookie", Value: "session=1; theme=dark", Enabled: true},
	}
	if !reflect.DeepEqual(req.Headers, wantHeaders) {
		t.Errorf("headers = %+v", req.Headers)
	}
	if !reflect.DeepEqual(req.Body, model.Body{Type: model.BodyRaw, Raw: `{"name":"it's"}`, ContentType: "application/json"}) {
		t.Errorf("body = %+v", req.Body)
	}
}

func TestParseFlags(t *testing.T) {
	cases := []struct {
		command string
		check   func(model.Request) bool
	}{
		{`curl example.com`, func(r model.Request) bool {
			return r.Method == model.MethodGet && r.URL == "example.com" && r.Body.Type == model.BodyNone
		}},
		{`curl -XPUT -sSL https://x.test`, func(r model.Request) bool { return r.Method == model.MethodPut && r.Options.FollowRedirects }},
		{`curl --no-location -k -m 2.5 -x http://proxy:8080 https://x.test`, func(r model.Request) bool {
			return !r.Options.FollowRedirects && !r.Options.VerifySSL && r.Options.TimeoutSeconds == 2.5 && r.Options.ProxyURL == "http://proxy:8080"
		}},
		{`curl -d 'a=1' -d 'b=two words' https://x.test`, func(r model.Request) bool {
			return r.Method == model.MethodPost && r.Body.Type == model.BodyRaw && r.Body.Raw == "a=1&b=two words"
		}},
		{`curl -d 'a=1' -d 'b=2' https://x.test`, func(r model.Request) bool {
			return r.Body.Type == model.BodyForm && len(r.Body.Form) == 2 && r.Body.Form[1].Value == "2"
		}},
		{`curl --data-urlencode 'msg=hello world' https://x.test`, func(r model.Request) bool {
			return r.Body.Type == model.BodyForm && r.Body.Form[0].Value == "hello world"
		}},
		{`curl -d '{"a": 1}' https://x.test`, func(r model.Request) bool {
			return r.Body.Type == model.BodyRaw && r.Body.ContentType == "application/json"
		}},
		{`curl --json '{"a": 1}' https://x.test`, func(r model.Request) bool {
			return r.Method == model.MethodPost && r.Body.ContentType == "application/json" && len(r.Headers) == 2
		}},
		{`curl -G -d q=go https://x.test/search`, func(r model.Request) bool {
			return r.Method == model.MethodGet && r.URL == "https://x.test/search?q=go" && r.Body.Type == model.BodyNone
		}},
		{`curl -I https://x.test`, func(r model.Request) bool { return r.Method == model.MethodHead }},
		{`curl -u ada:secret --digest https://x.test`, func(r model.Request) bool {
			return r.Auth == model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
		}},
		{`curl -H 'Authorization: Basic YWRhOnNlY3JldA==' https://x.test`, func(r model.Request) bool {
			return r.Auth == model.Auth{Type: model.AuthBasic, Username: "ada", Password: "secret"} && len(r.Headers) == 0
		}},
		{`curl -F name=ada -F role=admin https://x.test`, func(r model.Request) bool {
			return r.Method == model.MethodPost && r.Body.Type == model.BodyForm && len(r.Body.Form) == 2
		}},
		{`curl -A agent/1 -e https://ref.test -o out.json -w '%{http_code}' https://x.test`, func(r model.Request) bool {
			return r.URL == "https://x.test" && len(r.Headers) == 2 && r.Headers[0].Name == "User-Agent"
		}},
		{`curl --url https://x.test --request=DELETE`, func(r model.Request) bool { return r.URL == "https://x.test" }},
	}
	for _, c := range cases {
		req, err := Parse(c.command)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.command, err)
			continue
		}
		if !c.check(req) {
			t.Errorf("Parse(%q) = %+v", c.command, req)
		}
	}
	for _, bad := range []string{`wget x`, `curl -X`, `curl -H 'A: b'`, `curl -X FETCH https://x.test`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

// wire strips what a curl command can't carry, so requests can be compared
// after a round trip.
func wire(r model.Request) model.Request {
	r = r.Clone()
	r.Name, r.Description, r.File = "", "", ""
	r.Scripts = model.Scripts{}
	r.Options.AttachCookies = true
	r.Options.SubstituteBodyVariables = true
	var headers []model.KeyValue
	for _, h := range r.Headers {
		if h.Enabled {
			headers = append(headers, h)
		}
	}
	r.Headers = headers
	var query []model.KeyValue
	for _, q := range r.Query {
		if q.Enabled {
			query = append(query, q)
		}
	}
	r.Query = query
	if r.Body.Type == model.BodyForm {
		var form []model.KeyValue
		for _, f := range r.Body.Form {
			if f.Enabled {
				form = append(form, f)
			}
		}
		r.Body.Form = form
	}
	r.PathParams = nil
	return r
}

func TestRoundTripPosting2Collection(t *testing.T) {
	root, problems := collection.Dir{Root: "../../tests/sample-collections"}.Load()
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	n := 0
	root.Walk(func(_ *model.Collection, original model.Request) {
		n++
		for _, multiline := range []bool{false, true} {
			command := Format(original, FormatOptions{Multiline: multiline})
			back, err := Parse(command)
			if err != nil {
				t.Fatalf("%s: %v\n%s", original.File, err, command)
			}
			want := wire(original)
			want.URL = targetURL(original)
			if want.URL != original.URL {
				want.Query = nil
				back.Query = nil
			}
			if got := wire(back); !reflect.DeepEqual(got, want) {
				t.Errorf("%s changed in a round trip:\n%s\nbefore: %+v\nafter:  %+v", original.File, command, want, got)
			}
		}
	})
	if n == 0 {
		t.Fatal("no requests")
	}
}

func TestRoundTripEdgeCases(t *testing.T) {
	base := model.NewRequest()
	base.URL = "https://api.test/things?x=1"
	base.Query = []model.KeyValue{{Name: "x", Value: "1", Enabled: true}}

	xml := base.Clone()
	xml.Method = model.MethodPut
	xml.Body = model.Body{Type: model.BodyRaw, Raw: "<a b=\"1\">it's</a>", ContentType: "application/xml"}

	form := base.Clone()
	form.Method = model.MethodPatch
	form.Body = model.Body{Type: model.BodyForm, ContentType: "application/x-www-form-urlencoded", Form: []model.KeyValue{
		{Name: "q", Value: "a&b=c d", Enabled: true}, {Name: "emoji", Value: "é✓", Enabled: true},
	}}

	json := base.Clone()
	json.Body = model.Body{Type: model.BodyRaw, Raw: "{\n  \"multi\": \"line\"\n}", ContentType: "application/json"}
	json.Method = model.MethodGet // A GET with a body must keep its method.

	options := base.Clone()
	options.Method = model.MethodDelete
	options.Options.FollowRedirects = false
	options.Options.VerifySSL = false
	options.Options.TimeoutSeconds = 12.5
	options.Options.ProxyURL = "http://proxy.test:3128"
	options.Auth = model.Auth{Type: model.AuthBasic, Username: "ada", Password: "p@ss:word 'quoted'"}

	head := base.Clone()
	head.Method = model.MethodHead
	head.Auth = model.Auth{Type: model.AuthBearer, Token: "tok"}

	for name, original := range map[string]model.Request{"xml": xml, "form": form, "json": json, "options": options, "head": head} {
		command := Format(original, FormatOptions{Multiline: true, ExtraArgs: "--verbose"})
		back, err := Parse(command)
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, command)
		}
		want := wire(original)
		if name == "xml" {
			// A raw body without a Content-Type header gains one, so curl
			// (and the import) know what it is.
			want.Headers = append(want.Headers, model.KeyValue{Name: "Content-Type", Value: "application/xml", Enabled: true})
		}
		if got := wire(back); !reflect.DeepEqual(got, want) {
			t.Errorf("%s changed in a round trip:\n%s\nbefore: %+v\nafter:  %+v", name, command, want, got)
		}
	}
}

func TestFormat(t *testing.T) {
	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = "https://api.test/users"
	req.Headers = []model.KeyValue{{Name: "X-Off", Value: "no", Enabled: false}, {Name: "Accept", Value: "*/*", Enabled: true}}
	req.Body = model.Body{Type: model.BodyRaw, Raw: `{"name": "it's"}`, ContentType: "application/json"}
	got := Format(req, FormatOptions{})
	want := `curl https://api.test/users -H 'Accept: */*' --data-raw '{"name": "it'\''s"}' -L`
	if got != want {
		t.Errorf("single line:\n got %s\nwant %s", got, want)
	}
	got = Format(req, FormatOptions{Multiline: true, ExtraArgs: "-v"})
	want = "curl -v https://api.test/users \\\n  -H 'Accept: */*' \\\n  --data-raw '{\"name\": \"it'\\''s\"}' \\\n  -L"
	if got != want {
		t.Errorf("multi-line:\n got %s\nwant %s", got, want)
	}
}
