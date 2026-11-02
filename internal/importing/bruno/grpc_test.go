package bruno

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
)

// testdata/grpc follows Bruno's own gRPC fixtures, in tests/grpc of
// usebruno/bruno: a .bru file per method and the proto config in bruno.json.
func TestGRPCRequestsImportAsGRPC(t *testing.T) {
	result, err := Load("testdata/grpc")
	if err != nil {
		t.Fatal(err)
	}
	requests := map[string]model.Request{}
	for _, r := range result.Requests {
		requests[r.Name] = r
	}
	if len(requests) != 3 {
		t.Fatalf("imported %d requests, want 3; warnings %q", len(result.Requests), result.Warnings)
	}
	for _, c := range []struct {
		name, url string
		payload   model.GRPC
		headers   []model.KeyValue
		auth      model.Auth
	}{
		{
			name: "SayHello",
			url:  "grpc://localhost:50051",
			payload: model.GRPC{
				Method:  "hello.HelloService/SayHello",
				Message: "{\n  \"greeting\": \"${name}\"\n}",
				Protos:  model.ProtoSet{Files: []string{"protos/hello.proto"}, ImportPaths: []string{"protos"}},
			},
			headers: []model.KeyValue{{Name: "x-tenant", Value: "core", Enabled: true}, {Name: "test-bin", Value: "hello", Enabled: true}, {Name: "test", Value: "hello"}},
			auth:    model.Auth{Type: model.AuthBearer, Token: "${token}"},
		},
		{
			name: "LotOfGreetings",
			url:  "${host}",
			payload: model.GRPC{
				Method:  "hello.HelloService/LotsOfGreetings",
				Message: "[\n  {\n    \"greeting\": \"sortitus\"\n  },\n  {\n    \"greeting\": \"porro\"\n  }\n]",
			},
			headers: []model.KeyValue{{Name: "x-tenant", Value: "core", Enabled: true}},
			auth:    model.Auth{Type: model.AuthBearer, Token: "${token}"},
		},
		{
			// The proto lies outside every import path, so its own directory
			// is added for its imports to resolve.
			name: "GetProduct",
			url:  "grpcs://shop.example.com",
			payload: model.GRPC{
				Method:  "shop.ProductService/GetProduct",
				Message: "{\n  \"id\": 7\n}",
				Protos:  model.ProtoSet{Files: []string{"../protos/services/product.proto"}, ImportPaths: []string{"protos", "../protos/services"}},
			},
			headers: []model.KeyValue{{Name: "x-tenant", Value: "core", Enabled: true}},
			auth:    model.Auth{Type: model.AuthNone},
		},
	} {
		r, ok := requests[c.name]
		if !ok {
			t.Errorf("%s was not imported; warnings %q", c.name, result.Warnings)
			continue
		}
		if r.URL != c.url || !reflect.DeepEqual(r.Payload, c.payload) || !reflect.DeepEqual(r.Headers, c.headers) || r.Auth != c.auth {
			t.Errorf("%s imported as\n url %q\n payload %#v\n headers %+v\n auth %+v\nwant\n url %q\n payload %#v\n headers %+v\n auth %+v",
				c.name, r.URL, r.Payload, r.Headers, r.Auth, c.url, c.payload, c.headers, c.auth)
		}
		data, err := collection.MarshalRequest(r)
		if err != nil {
			t.Fatal(err)
		}
		back, err := collection.ParseRequest(data, r.File)
		if err != nil || !reflect.DeepEqual(back.Payload, c.payload) {
			t.Errorf("%s: saved file didn't load back: %v\n%s", c.name, err, data)
		}
	}
	warnings := strings.Join(result.Warnings, "\n")
	for _, want := range []string{
		"HelloService/SayHello.bru: proto file protos/hello.proto is not copied",
		"HelloService/GetProduct.bru: proto file ../protos/services/product.proto is not copied",
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings %q lack %q", result.Warnings, want)
		}
	}
	if strings.Contains(warnings, "unsupported") || strings.Contains(warnings, "skipped") {
		t.Errorf("gRPC collection imported with warnings %q", result.Warnings)
	}
}

func TestGRPCRequestsWarnWhenInheritedDigestAuthIsDropped(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		"bruno.json":     `{"name": "c"}`,
		"collection.bru": "auth {\n  mode: digest\n}\nauth:digest {\n  username: ada\n  password: secret\n}\n",
		"call.bru":       "meta {\n  type: grpc\n}\ngrpc {\n  url: localhost:50051\n  method: /a.B/C\n  auth: inherit\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests) != 1 || result.Requests[0].Auth.Type != model.AuthNone {
		t.Fatalf("requests %+v", result.Requests)
	}
	if want := "call.bru: gRPC requests can't use digest authentication; credentials were not imported"; !strings.Contains(strings.Join(result.Warnings, "\n"), want) {
		t.Fatalf("warnings %q lack %q", result.Warnings, want)
	}
}

func TestGRPCImportWarnings(t *testing.T) {
	const message = "body:grpc {\n  name: message 1\n  content: '''\n    {}\n  '''\n}\n"
	for _, c := range []struct{ name, input, want string }{
		{"unary method with two messages", "meta {\n  type: grpc\n}\ngrpc {\n  url: localhost:50051\n  method: /a.B/C\n  methodType: unary\n}\n" + message + message, "unary method sends one message; imported only the first"},
		{"API key in query parameters", "meta {\n  type: grpc\n}\ngrpc {\n  url: localhost:50051\n  method: /a.B/C\n  auth: apikey\n}\nauth:apikey {\n  key: k\n  value: v\n  placement: queryparams\n}\n" + message, "gRPC has no query parameters; API key was not imported"},
		{"OAuth 2", "meta {\n  type: grpc\n}\ngrpc {\n  url: localhost:50051\n  method: /a.B/C\n  auth: oauth2\n}\n" + message, "unsupported authentication oauth2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			result, err := Parse([]byte(c.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Requests) != 1 {
				t.Fatalf("imported %d requests; warnings %q", len(result.Requests), result.Warnings)
			}
			if !strings.Contains(strings.Join(result.Warnings, "\n"), c.want) {
				t.Fatalf("warnings %q lack %q", result.Warnings, c.want)
			}
			if r := result.Requests[0]; r.Auth.Type != model.AuthNone || len(r.Headers) != 0 {
				t.Fatalf("lossy auth left %+v, headers %+v", r.Auth, r.Headers)
			}
		})
	}
}
