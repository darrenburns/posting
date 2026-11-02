package curl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
)

func TestParseJSONFileWithoutReading(t *testing.T) {
	req, err := Parse(`curl --json @missing.json https://api.test`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Body.Type != model.BodyFile || req.Body.File != "missing.json" || req.Body.ContentType != "application/json" || headerValue(req.Headers, "Accept") != "application/json" || req.Method != model.MethodPost {
		t.Fatalf("JSON file imported as %+v", req)
	}
}

func TestJSONFileCurlWire(t *testing.T) {
	server := bodyHeaderServer(t)
	root := t.TempDir()
	file := filepath.Join(root, "payload.json")
	for _, flags := range []string{"", `-H 'Content-Type: application/vnd.test+json' -H 'Accept: application/vnd.test+json'`, `-H 'Content-Type;' -H 'Accept;'`} {
		req, err := Parse("curl --json @payload.json " + flags + " " + Quote(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		for _, content := range []string{"{\"name\":\"first\"}\n", "{\"name\":\"updated\"}\n\n"} {
			if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			actual := runBodyHeaderCurl(t, "curl --json "+Quote("@"+file)+" "+flags+" "+Quote(server.URL))
			http := client.NewHTTP("", client.TLSSettings{})
			http.Root = root
			resp, err := http.Send(context.Background(), client.Call{Request: req})
			if err != nil {
				t.Fatal(err)
			}
			var got bodyHeaderRequest
			if err := json.Unmarshal(resp.Body, &got); err != nil {
				t.Fatal(err)
			}
			if got.Body != content || got.Body != actual.Body || !reflect.DeepEqual(got.Headers.Values("Content-Type"), actual.Headers.Values("Content-Type")) || !reflect.DeepEqual(got.Headers.Values("Accept"), actual.Headers.Values("Accept")) {
				t.Fatalf("Posting sent %+v, curl sent %+v", got, actual)
			}
		}
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	req, err := Parse("curl --json " + Quote("@"+file) + " " + Quote(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.NewHTTP("", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req}); err == nil {
		t.Fatal("sending a missing JSON file succeeded")
	}
}

func TestParseRejectsUnsupportedJSONFileCombinations(t *testing.T) {
	for _, flags := range []string{
		"--json @", "--json @-", "--json @one --json @two", "--json @one --data-binary @two",
		"--data-binary @one --json @two", "--json @one --json '{}'", "--json '{}' --json @one",
		"--json @one --data x", "--json @one -F x=y", "--json @one -G",
	} {
		if _, err := Parse("curl " + flags + " https://api.test"); err == nil {
			t.Errorf("accepted %s", flags)
		}
	}
}

func TestJSONDataKeepsExplicitContentType(t *testing.T) {
	server := bodyHeaderServer(t)
	command := "curl --json '{\"name\":\"a b\"}' -H 'Content-Type: application/x-www-form-urlencoded' " + Quote(server.URL)
	want := runBodyHeaderCurl(t, command)
	req, err := Parse(command)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.NewHTTP("", client.TLSSettings{}).Send(context.Background(), client.Call{Request: req})
	if err != nil {
		t.Fatal(err)
	}
	var got bodyHeaderRequest
	if err := json.Unmarshal(resp.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Body != `{"name":"a b"}` || got.Body != want.Body || got.Headers.Get("Content-Type") != want.Headers.Get("Content-Type") {
		t.Fatalf("Posting sent %+v, curl sent %+v", got, want)
	}
}
