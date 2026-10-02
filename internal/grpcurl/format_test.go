package grpcurl

import (
	"testing"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
)

func request(url string, payload model.GRPC) model.Request {
	r := model.GRPCKind.New()
	r.URL = url
	r.Payload = payload
	return r
}

func TestFormat(t *testing.T) {
	r := request("localhost:50051", model.GRPC{Method: "library.v1.Library/GetBook", Message: "{\n  \"isbn\": \"1\"\n}"})
	r.Headers = []model.KeyValue{{Name: "X-Tenant", Value: "core", Enabled: true}, {Name: "x-off", Value: "no"}}
	r.Auth = model.Auth{Type: model.AuthBearer, Token: "t0k"}
	got := Format(r, FormatOptions{Multiline: true})
	want := `grpcurl \
  -plaintext \
  -H 'x-tenant: core' \
  -H 'authorization: Bearer t0k' \
  -d '{
  "isbn": "1"
}' \
  -max-time 5 \
  localhost:50051 library.v1.Library/GetBook`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFormatTargets(t *testing.T) {
	for _, c := range []struct {
		url    string
		verify bool
		want   string
	}{
		{"grpcs://api.example.com", true, "grpcurl -max-time 5 api.example.com:443 a.B/C"},
		{"api.example.com:8443", false, "grpcurl -insecure -max-time 5 api.example.com:8443 a.B/C"},
		{"grpc://api.example.com:80", true, "grpcurl -plaintext -max-time 5 api.example.com:80 a.B/C"},
	} {
		r := request(c.url, model.GRPC{Method: "/a.B/C"})
		r.Options.VerifySSL = c.verify
		if got := Format(r, FormatOptions{}); got != c.want {
			t.Errorf("%s: got %s\nwant %s", c.url, got, c.want)
		}
	}
}

func TestFormatStreamsAndAuth(t *testing.T) {
	r := request("localhost:1", model.GRPC{Method: "a.B/C", Message: "[\n  {\"n\": 1},\n  {\"n\": 2}\n]"})
	r.Auth = model.Auth{Type: model.AuthBasic, Username: "ada", Password: "pw"}
	want := "grpcurl -plaintext -H 'authorization: Basic YWRhOnB3' -d '{\"n\":1}\n{\"n\":2}' -max-time 5 localhost:1 a.B/C"
	if got := Format(r, FormatOptions{}); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	words, err := curl.Split(Format(r, FormatOptions{Multiline: true}))
	if err != nil || words[5] != "{\"n\":1}\n{\"n\":2}" {
		t.Fatalf("the multiline command splits as %q, %v", words, err)
	}
}

func TestFormatProtoFiles(t *testing.T) {
	r := request("localhost:1", model.GRPC{Method: "a.B/C", Protos: model.ProtoSet{
		Files:       []string{"protos/acme/a.proto", "gen/all.protoset"},
		ImportPaths: []string{"protos"},
	}})
	want := "grpcurl -plaintext -max-time 5 -protoset /c/gen/all.protoset -import-path /c/protos -proto acme/a.proto localhost:1 a.B/C"
	if got := Format(r, FormatOptions{Root: "/c"}); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}

	r.Payload = model.GRPC{Method: "a.B/C", Protos: model.ProtoSet{Files: []string{"a.proto"}}}
	want = "grpcurl -plaintext -max-time 5 -import-path /c -proto a.proto localhost:1 a.B/C"
	if got := Format(r, FormatOptions{Root: "/c"}); got != want {
		t.Fatalf("without import paths the collection is one: got %s", got)
	}
}
