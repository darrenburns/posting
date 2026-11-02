package collection

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

// testdata/golden holds what MarshalRequest wrote for each Posting 2 sample
// file before request kinds existed.
func TestPosting2FilesMarshalAsBefore(t *testing.T) {
	golden := "testdata/golden"
	count := 0
	err := filepath.WalkDir(sampleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, FileSuffix) {
			return err
		}
		rel, _ := filepath.Rel(sampleDir, path)
		data, _ := os.ReadFile(path)
		req, err := ParseRequest(data, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got, err := MarshalRequest(req)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		want, err := os.ReadFile(filepath.Join(golden, rel))
		if err != nil {
			t.Fatalf("%s: no golden file: %v", rel, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s re-saves differently:\n got\n%s\nwant\n%s", rel, got, want)
		}
		count++
		return nil
	})
	if err != nil || count != 17 {
		t.Fatalf("checked %d files, err %v", count, err)
	}
}

func TestEveryKindRoundTripsThroughFiles(t *testing.T) {
	for _, k := range model.Kinds {
		t.Run(string(k.ID), func(t *testing.T) {
			want := k.Example()
			data, err := MarshalRequest(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseRequest(data, want.File)
			if err != nil {
				t.Fatalf("%v\n%s", err, data)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip:\n got %+v\nwant %+v\n%s", got, want, data)
			}
			again, _ := MarshalRequest(got)
			if string(again) != string(data) {
				t.Fatalf("re-marshal changed the file:\n%s\n%s", data, again)
			}
		})
	}
}

func TestGraphQLFile(t *testing.T) {
	req := model.GraphQLKind.Example()
	req.Method = model.MethodPut
	req.Body = model.Body{Type: model.BodyRaw, Raw: "stray"}
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	want := `name: Get user
description: Fetch a user through the GraphQL API.
kind: graphql
url: ${BASE_URL}/graphql
graphql:
  query: |
    query User($id: ID!) {
      user(id: $id) {
        name
        email
      }
    }
  variables: |
    {"id": "${USER_ID}"}
  operation_name: User
auth:
  type: bearer_token
  bearer_token:
    token: ${API_TOKEN}
headers:
  - name: X-Request-Source
    value: posting
options:
  follow_redirects: false
`
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
}

func TestParseRejectsFilesThatDontFitTheirKind(t *testing.T) {
	for _, c := range []struct{ name, file, wantInError string }{
		{"unknown kind", "kind: carrier-pigeon\nurl: https://x\n", "carrier-pigeon"},
		{"graphql with a method", "kind: graphql\nmethod: PUT\nurl: https://x\n", "method"},
		{"graphql with a body", "kind: graphql\nurl: https://x\nbody:\n  content: hi\n", "body"},
		{"graphql block on http", "url: https://x\ngraphql:\n  query: '{ a }'\n", "graphql"},
		{"unknown key in the graphql block", "kind: graphql\nurl: https://x\ngraphql:\n  query: '{ a }'\n  operationName: A\n", "operationName"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(c.file), "")
			if err == nil {
				t.Fatalf("parsed as %+v", req)
			}
			if !strings.Contains(err.Error(), c.wantInError) {
				t.Fatalf("error %q doesn't name %q", err, c.wantInError)
			}
		})
	}
}

func TestParseExplicitHTTPKind(t *testing.T) {
	req, err := ParseRequest([]byte("kind: http\nmethod: POST\nurl: https://x\n"), "")
	if err != nil || req.Kind() != model.HTTPKind || req.Method != model.MethodPost {
		t.Fatalf("kind: http parsed as %+v, %v", req, err)
	}
}

func TestParseKindIgnoresCase(t *testing.T) {
	req, err := ParseRequest([]byte("kind: GraphQL\nurl: https://x\ngraphql:\n  query: '{ a }'\n"), "")
	if err != nil || req.Payload != (model.GraphQL{Query: "{ a }"}) {
		t.Fatalf("kind: GraphQL parsed as %+v, %v", req, err)
	}
}

func TestParseKeepsIgnoringUnknownTopLevelKeys(t *testing.T) {
	req, err := ParseRequest([]byte("url: https://x\nfuture_key: 1\n"), "")
	if err != nil || req.URL != "https://x" {
		t.Fatalf("a file with an unknown top-level key parsed as %+v, %v", req, err)
	}
}

func TestParseGraphQLWithoutBlock(t *testing.T) {
	req, err := ParseRequest([]byte("kind: graphql\nurl: https://x\n"), "")
	if err != nil || req.Payload != (model.GraphQL{}) {
		t.Fatalf("a graphql file with no block is an empty GraphQL request, got %+v, %v", req, err)
	}
}
