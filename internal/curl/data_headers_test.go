package curl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"testing"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
)

type bodyHeaderRequest struct {
	Headers http.Header
	Body    string
}

func bodyHeaderServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(bodyHeaderRequest{Headers: r.Header, Body: string(body)})
	}))
	t.Cleanup(server.Close)
	return server
}

func runBodyHeaderCurl(t *testing.T, command string) bodyHeaderRequest {
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
	var got bodyHeaderRequest
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("curl response: %v\n%s", err, out)
	}
	return got
}

func TestFormatEmptyHeaderCurlWire(t *testing.T) {
	server := bodyHeaderServer(t)
	req := model.NewRequest()
	req.URL = server.URL
	req.Headers = []model.KeyValue{
		{Name: "X-Empty", Enabled: true},
		{Name: "X-Present", Value: "value", Enabled: true},
		{Name: "X-Disabled", Enabled: false},
	}
	for _, multiline := range []bool{false, true} {
		command := Format(req, FormatOptions{Multiline: multiline})
		got := runBodyHeaderCurl(t, command)
		if values, present := got.Headers["X-Empty"]; !present || !reflect.DeepEqual(values, []string{""}) {
			t.Errorf("exported curl X-Empty = %q, present = %v; want present empty header\n%s", values, present, command)
		}
		if got.Headers.Get("X-Present") != "value" {
			t.Errorf("exported curl X-Present = %q, want value", got.Headers.Get("X-Present"))
		}
		if _, present := got.Headers["X-Disabled"]; present {
			t.Error("exported curl sent disabled header")
		}
		parsed, err := Parse(command)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(parsed.Headers, req.Headers[:2]) {
			t.Errorf("imported headers = %+v, want %+v", parsed.Headers, req.Headers[:2])
		}
	}
}

func TestParseDataChunksCurlWire(t *testing.T) {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl executable is needed for wire interoperability checks")
	}
	version, err := exec.Command(curlPath, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	var major, minor, patch int
	if _, err := fmt.Sscanf(string(version), "curl %d.%d.%d", &major, &minor, &patch); err != nil {
		t.Fatalf("cannot read curl version: %v\n%s", err, version)
	}
	legacyEmptyData := major < 8 || (major == 8 && minor < 6)

	server := bodyHeaderServer(t)
	for _, tc := range []struct {
		name, flags, want, legacyWant string
	}{
		{"repeated_json", `--json '{"name":' --json '"Posting"}'`, `{"name":"Posting"}`, ""},
		{"data_then_json", `--data first --json second`, "firstsecond", ""},
		{"json_then_data", `--json first --data second`, "first&second", ""},
		{"alternating", `--data first --json second --data-raw third --json fourth`, "firstsecond&thirdfourth", ""},
		{"leading_empty_json", `--json '' --data second`, "second", "&second"},
		{"leading_empty_data", `--data '' --json second`, "second", ""},
		{"empty_middle_json", `--data first --json '' --data third`, "first&third", ""},
		{"json_then_binary", `--json first --data-binary second`, "first&second", ""},
		{"json_then_encoded", `--json first --data-urlencode 'name=a b'`, "first&name=a+b", ""},
		{"repeated_data", `--data 'a=1' --data 'b=2'`, "a=1&b=2", ""},
		{"leading_empty_plain_data", `--data '' --data 'a=1'`, "a=1", "&a=1"},
		{"multiple_leading_empty_data", `--data '' --data '' --data third`, "third", "&&third"},
		{"empty_middle_data", `--data first --data '' --data third`, "first&&third", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := "curl " + tc.flags + " " + Quote(server.URL)
			actual := runBodyHeaderCurl(t, command)
			// curl 8.6.0 changed the separator check from a buffer pointer
			// to its length: https://github.com/curl/curl/commit/1f4433dad4cb6fbb5c6a85c95ee1fcd8891a4129.
			// Only curl's expectation varies; Posting follows modern curl.
			curlWant := tc.want
			if legacyEmptyData && tc.legacyWant != "" {
				curlWant = tc.legacyWant
			}
			if actual.Body != curlWant {
				t.Errorf("curl %d.%d.%d body = %q, want %q", major, minor, patch, actual.Body, curlWant)
			}
			req, err := Parse(command)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.NewHTTP("", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req})
			if err != nil {
				t.Fatal(err)
			}
			var imported bodyHeaderRequest
			if err := json.Unmarshal(response.Body, &imported); err != nil {
				t.Fatalf("Posting response: %v\n%s", err, response.Body)
			}
			if imported.Body != tc.want {
				t.Errorf("imported request body = %q, want %q", imported.Body, tc.want)
			}
		})
	}
}
