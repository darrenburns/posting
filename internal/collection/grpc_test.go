package collection

import (
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

func TestGRPCFile(t *testing.T) {
	req := model.GRPCKind.Example()
	req.Method = model.MethodPut
	req.Options.ProxyURL = "http://proxy:8080"
	req.Options.FollowRedirects = false
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	want := `name: Get user over gRPC
description: Fetch a user through the gRPC API.
kind: grpc
url: localhost:50051
grpc:
  method: acme.users.v1.UserService/GetUser
  message: |
    {"id": "${USER_ID}"}
  proto:
    files:
      - protos/acme/users/v1/users.proto
    import_paths:
      - protos
auth:
  type: bearer_token
  bearer_token:
    token: ${API_TOKEN}
headers:
  - name: x-tenant
    value: core
options:
  verify_ssl: false
  timeout: 10
`
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
}

func TestGRPCFileWithReflectionHasNoProtoBlock(t *testing.T) {
	req := model.GRPCKind.New()
	req.URL = "localhost:50051"
	req.Payload = model.GRPC{Method: "library.v1.Library/GetBook", Message: `{"isbn": "1"}`}
	data, err := MarshalRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	want := "kind: grpc\nurl: localhost:50051\ngrpc:\n  method: library.v1.Library/GetBook\n  message: '{\"isbn\": \"1\"}'\n"
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
	got, err := ParseRequest(data, "")
	if err != nil || !reflect.DeepEqual(got.Payload, req.Payload) {
		t.Fatalf("parsed %+v, %v", got.Payload, err)
	}
}

func TestParseGRPC(t *testing.T) {
	for _, c := range []struct {
		name, file string
		want       model.GRPC
	}{
		{"no block", "kind: grpc\nurl: localhost:1\n", model.GRPC{}},
		{"kind ignores case", "kind: gRPC\nurl: localhost:1\ngrpc:\n  method: a.B/C\n", model.GRPC{Method: "a.B/C"}},
		{"empty proto lists read as reflection", "kind: grpc\ngrpc:\n  proto:\n    files: []\n    import_paths: []\n", model.GRPC{}},
		{"proto files without import paths", "kind: grpc\ngrpc:\n  proto:\n    files: [a.protoset]\n", model.GRPC{Protos: model.ProtoSet{Files: []string{"a.protoset"}}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(c.file), "")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(req.Payload, c.want) {
				t.Fatalf("payload = %#v, want %#v", req.Payload, c.want)
			}
		})
	}
}

func TestParseGRPCKeepsTheOptionsItUses(t *testing.T) {
	req, err := ParseRequest([]byte("kind: grpc\nurl: api.test:443\noptions:\n  verify_ssl: false\n  substitute_body_variables: false\n  timeout: 30\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if o := req.Options; o.VerifySSL || o.SubstituteBodyVariables || o.TimeoutSeconds != 30 {
		t.Fatalf("options = %+v", o)
	}
}

func TestParseRejectsGRPCFilesWithHTTPFields(t *testing.T) {
	for _, c := range []struct{ name, file, wantInError string }{
		{"method", "kind: grpc\nmethod: POST\nurl: localhost:1\n", "method"},
		{"body", "kind: grpc\nurl: localhost:1\nbody:\n  content: hi\n", "body"},
		{"query", "kind: grpc\nurl: localhost:1\nparams:\n  - name: a\n    value: b\n", "params"},
		{"path params", "kind: grpc\nurl: localhost:1\npath_params:\n  - name: a\n    value: b\n", "path_params"},
		{"follow redirects", "kind: grpc\nurl: localhost:1\noptions:\n  follow_redirects: false\n", "follow_redirects"},
		{"attach cookies", "kind: grpc\nurl: localhost:1\noptions:\n  attach_cookies: true\n", "attach_cookies"},
		{"proxy", "kind: grpc\nurl: localhost:1\noptions:\n  proxy_url: http://p:1\n", "proxy_url"},
		{"digest auth", "kind: grpc\nurl: localhost:1\nauth:\n  type: digest\n  digest:\n    username: a\n    password: b\n", "digest"},
		{"grpc block on http", "url: https://x\ngrpc:\n  method: a.B/C\n", "grpc"},
		{"grpc block on graphql", "kind: graphql\nurl: https://x\ngrpc:\n  method: a.B/C\n", "grpc"},
		{"graphql block on grpc", "kind: grpc\nurl: localhost:1\ngraphql:\n  query: '{ a }'\n", "graphql"},
		{"unknown key in the grpc block", "kind: grpc\ngrpc:\n  service: a.B\n", "service"},
		{"unknown key in the proto block", "kind: grpc\ngrpc:\n  proto:\n    protoset: a.pb\n", "protoset"},
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

func TestParseKeepsHTTPOnlyOptionsForGraphQL(t *testing.T) {
	req, err := ParseRequest([]byte("kind: graphql\nurl: https://x\noptions:\n  attach_cookies: false\n  proxy_url: http://p:1\n"), "")
	if err != nil || req.Options.AttachCookies || req.Options.ProxyURL != "http://p:1" {
		t.Fatalf("GraphQL options parsed as %+v, %v", req.Options, err)
	}
	req, err = ParseRequest([]byte("kind: graphql\nurl: https://x\nauth:\n  type: digest\n  digest:\n    username: a\n    password: b\n"), "")
	if want := (model.Auth{Type: model.AuthDigest, Username: "a", Password: "b"}); err != nil || req.Auth != want {
		t.Fatalf("GraphQL digest auth parsed as %+v, %v", req.Auth, err)
	}
}
