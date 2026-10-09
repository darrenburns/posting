package grpcurl

import (
	"testing"

	"github.com/darrenburns/posting/v3/internal/curl"
	"github.com/darrenburns/posting/v3/internal/model"
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
		{"unix:///run/books.sock", true, "grpcurl -plaintext -unix -max-time 5 /run/books.sock a.B/C"},
		{"unix:${SOCKET}", true, "grpcurl -plaintext -unix -max-time 5 '${SOCKET}' a.B/C"},
	} {
		r := request(c.url, model.GRPC{Method: "/a.B/C"})
		r.Options.VerifySSL = c.verify
		if got := Format(r, FormatOptions{}); got != c.want {
			t.Errorf("%s: got %s\nwant %s", c.url, got, c.want)
		}
	}
}

func TestFormatAuthority(t *testing.T) {
	r := request("grpcs://10.0.0.5", model.GRPC{Method: "a.B/C", Authority: "books.internal"})
	r.Options.VerifySSL = true
	want := "grpcurl -authority books.internal -max-time 5 10.0.0.5:443 a.B/C"
	if got := Format(r, FormatOptions{}); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
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

func TestFormatAuthReplacesAnAuthorizationRow(t *testing.T) {
	r := request("localhost:1", model.GRPC{Method: "a.B/C"})
	r.Headers = []model.KeyValue{{Name: "Authorization", Value: "stale", Enabled: true}, {Name: "x-a", Value: "1", Enabled: true}}
	r.Auth = model.Auth{Type: model.AuthBearer, Token: "t0k"}
	want := "grpcurl -plaintext -H 'x-a: 1' -H 'authorization: Bearer t0k' -max-time 5 localhost:1 a.B/C"
	if got := Format(r, FormatOptions{}); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestFormatMatchesProtoFilesToImportPathsOnceAbsolute(t *testing.T) {
	r := request("localhost:1", model.GRPC{Method: "a.B/C", Protos: model.ProtoSet{
		Files:       []string{"/c/protos/acme/a.proto", "./protos/acme/b.proto"},
		ImportPaths: []string{"protos"},
	}})
	want := "grpcurl -plaintext -max-time 5 -import-path /c/protos -proto acme/a.proto -proto acme/b.proto localhost:1 a.B/C"
	if got := Format(r, FormatOptions{Root: "/c"}); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestFormatTLSFiles(t *testing.T) {
	r := request("grpcs://api.example.com", model.GRPC{Method: "a.B/C"})
	r.Options.VerifySSL = true
	opts := FormatOptions{CACert: "/ssl/ca.pem", Cert: "/ssl/me.pem", Key: "/ssl/me.key"}
	want := "grpcurl -cacert /ssl/ca.pem -cert /ssl/me.pem -key /ssl/me.key -max-time 5 api.example.com:443 a.B/C"
	if got := Format(r, opts); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}

	opts = FormatOptions{Cert: "/ssl/both.pem"}
	want = "grpcurl -cert /ssl/both.pem -key /ssl/both.pem -max-time 5 api.example.com:443 a.B/C"
	if got := Format(r, opts); got != want {
		t.Fatalf("a certificate file without a key file holds both: got %s", got)
	}

	r.URL = "localhost:1"
	if got := Format(r, FormatOptions{CACert: "/ssl/ca.pem"}); got != "grpcurl -plaintext -max-time 5 localhost:1 a.B/C" {
		t.Fatalf("a plaintext call has no use for TLS files: got %s", got)
	}
}

func TestFormatUnresolvedAddressIsKeptAsWrittenAndOnlyAnExplicitSchemeChoosesPlaintext(t *testing.T) {
	for _, c := range []struct{ url, want string }{
		{"${HOST}", "grpcurl -max-time 5 '${HOST}' a.B/C"},
		{"${HOST}:${PORT}", "grpcurl -max-time 5 '${HOST}:${PORT}' a.B/C"},
		{"grpc://${HOST}:${PORT}", "grpcurl -plaintext -max-time 5 '${HOST}:${PORT}' a.B/C"},
		{"grpcs://${HOST}", "grpcurl -max-time 5 '${HOST}' a.B/C"},
	} {
		r := request(c.url, model.GRPC{Method: "a.B/C"})
		r.Options.VerifySSL = true
		if got := Format(r, FormatOptions{}); got != c.want {
			t.Errorf("%s: got  %s\nwant %s", c.url, got, c.want)
		}
	}
}
