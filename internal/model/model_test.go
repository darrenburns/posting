package model

import (
	"reflect"
	"testing"
)

func lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) { v, ok := values[name]; return v, ok }
}

func TestFindVariables(t *testing.T) {
	refs := FindVariables("${BASE}/users/$ID?x=$$literal&y=${bad name}")
	want := []VariableRef{{Name: "BASE", Start: 0, End: 7}, {Name: "ID", Start: 14, End: 17}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("FindVariables = %+v, want %+v", refs, want)
	}
}

func TestSubstitute(t *testing.T) {
	got := Substitute("${BASE}/u/$ID/$MISSING?cost=$$5", lookup(map[string]string{"BASE": "http://x", "ID": "7"}))
	want := "http://x/u/7/$MISSING?cost=$5"
	if got != want {
		t.Fatalf("Substitute = %q, want %q", got, want)
	}
}

func TestPathParamNames(t *testing.T) {
	cases := map[string][]string{
		"https://example.com/users/:id/posts/:post?x=:no": {"id", "post"},
		"${BASE}/users/:id/::literal":                     {"id"},
		"https://example.com":                             nil,
		"/a/:id/b/:id":                                    {"id"},
	}
	for input, want := range cases {
		if got := PathParamNames(input); !reflect.DeepEqual(got, want) {
			t.Errorf("PathParamNames(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestResolvePathParams(t *testing.T) {
	params := []KeyValue{{Name: "id", Value: "a b"}, {Name: "empty"}}
	got := ResolvePathParams("https://example.com/users/:id/:empty/::raw?q=:id", params)
	want := "https://example.com/users/a%20b/:empty/:raw?q=:id"
	if got != want {
		t.Fatalf("ResolvePathParams = %q, want %q", got, want)
	}
}

func TestCurl(t *testing.T) {
	req := NewRequest()
	req.Method = MethodPost
	req.URL = "${BASE}/users/:id"
	req.PathParams = []KeyValue{{Name: "id", Value: "42", Enabled: true}}
	req.Headers = []KeyValue{
		{Name: "Content-Type", Value: "application/json", Enabled: true},
		{Name: "X-Off", Value: "no", Enabled: false},
	}
	req.Auth = Auth{Type: AuthBearer, Token: "${TOKEN}"}
	req.Body = Body{Type: BodyRaw, Raw: `{"name": "it's"}`}
	got := Curl(req, lookup(map[string]string{"BASE": "https://api.test", "TOKEN": "abc"}))
	want := `curl -X POST https://api.test/users/42 -H 'Content-Type: application/json' -H 'Authorization: Bearer abc' --data-raw '{"name": "it'\''s"}' -L`
	if got != want {
		t.Fatalf("Curl =\n%s\nwant\n%s", got, want)
	}
}

func TestSortCollection(t *testing.T) {
	c := &Collection{Requests: []Request{
		{Name: "b", Method: MethodDelete},
		{Name: "z", Method: MethodGet},
		{Name: "a", Method: MethodGet},
		{Name: "h", Method: MethodHead},
	}}
	c.Sort()
	var order []string
	for _, r := range c.Requests {
		order = append(order, string(r.Method)+" "+r.Name)
	}
	want := []string{"GET a", "GET z", "DELETE b", "HEAD h"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("sorted = %v, want %v", order, want)
	}
}

func TestIsSensitiveName(t *testing.T) {
	for name, want := range map[string]bool{"API_TOKEN": true, "db_password": true, "BASE_URL": false, "apiKey": true} {
		if got := IsSensitiveName(name); got != want {
			t.Errorf("IsSensitiveName(%q) = %v, want %v", name, got, want)
		}
	}
}
