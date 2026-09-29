package curl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"reflect"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

func specialForm() []model.KeyValue {
	return []model.KeyValue{
		{Name: "a&b", Value: "one & two", Enabled: true},
		{Name: "a=b", Value: "three=four", Enabled: true},
		{Name: "a+b", Value: "five+six", Enabled: true},
		{Name: "a%26b", Value: "seven", Enabled: true},
		{Name: "café name", Value: "✓", Enabled: true},
	}
}

func TestFormatFormNamesRoundTrip(t *testing.T) {
	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = "https://example.com"
	req.Body = model.Body{Type: model.BodyForm, Form: specialForm()}
	command := Format(req, FormatOptions{})
	back, err := Parse(command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Body.Form, req.Body.Form) {
		t.Fatalf("form fields changed: got %+v, want %+v\ncommand: %s", back.Body.Form, req.Body.Form, command)
	}
}

func TestParseGetDataBeforeFragment(t *testing.T) {
	for _, tc := range []struct {
		name, suffix, want string
	}{
		{"fragment", "#section", "?q=hello+world#section"},
		{"query_and_fragment", "?existing=1#section", "?existing=1&q=hello+world#section"},
		{"question_in_fragment", "#section?details", "?q=hello+world#section?details"},
		{"empty_fragment", "#", "?q=hello+world#"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := Parse("curl -G --data-urlencode 'q=hello world' " + Quote("https://example.com/search"+tc.suffix))
			if err != nil {
				t.Fatal(err)
			}
			if want := "https://example.com/search" + tc.want; req.URL != want {
				t.Errorf("imported URL = %q, want %q", req.URL, want)
			}
			var q string
			for _, kv := range req.Query {
				if kv.Name == "q" {
					q = kv.Value
				}
			}
			if q != "hello world" {
				t.Errorf("imported query = %+v, want q=hello world", req.Query)
			}
		})
	}
}

// These tests exercise curl itself so a matching bug in Parse and Format cannot
// hide a change to what the server receives. Only fixture commands are executed,
// with argv (never a shell), curl's config disabled, and a loopback server.
type wireRequest struct {
	Method string
	Query  url.Values
	Form   url.Values
}

func wireServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(wireRequest{r.Method, r.URL.Query(), r.PostForm})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runFixtureCurl(t *testing.T, command string) wireRequest {
	t.Helper()
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl executable is needed for wire interoperability checks")
	}
	args, err := Split(command)
	if err != nil {
		t.Fatal(err)
	}
	args = append([]string{"-q", "--silent", "--show-error", "--noproxy", "*", "--max-time", "5"}, args[1:]...)
	out, err := exec.Command(curlPath, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("curl failed: %v\n%s", err, out)
	}
	var got wireRequest
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("curl response: %v\n%s", err, out)
	}
	return got
}

func TestFormatFormNamesCurlWire(t *testing.T) {
	srv := wireServer(t)
	req := model.NewRequest()
	req.URL, req.Method = srv.URL, model.MethodPost
	req.Body = model.Body{Type: model.BodyForm, Form: specialForm()}
	want := url.Values{}
	for _, f := range req.Body.Form {
		want.Add(f.Name, f.Value)
	}
	got := runFixtureCurl(t, Format(req, FormatOptions{}))
	if got.Method != "POST" || !reflect.DeepEqual(got.Form, want) {
		t.Fatalf("exported curl wire request = %+v; want POST form %v", got, want)
	}
}

func TestParseGetDataFragmentCurlWire(t *testing.T) {
	srv := wireServer(t)
	command := "curl -G --data-urlencode 'q=hello world' " + Quote(srv.URL+"/search?existing=1#section?details")
	want := runFixtureCurl(t, command)
	req, err := Parse(command)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := model.Resolve(req, model.MapLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	r, err := http.NewRequest(string(resolved.Method), resolved.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := srv.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var got wireRequest
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imported request wire = %+v; actual curl wire = %+v", got, want)
	}
}
