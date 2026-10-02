package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGRPCIsNotCarriedOverHTTP(t *testing.T) {
	if GRPCKind.OverHTTP() {
		t.Fatal("gRPC has its own transport")
	}
	if wire, ok := Lower(GRPCKind.Example()); ok {
		t.Fatalf("Lower turned a gRPC request into %+v", wire)
	}
}

func TestNormalizeClearsFieldsGRPCDoesNotUse(t *testing.T) {
	r := GRPCKind.Example()
	r.Method = MethodPost
	r.Body = Body{Type: BodyRaw, Raw: "stray", ContentType: "text/plain"}
	r.Query = []KeyValue{{Name: "a", Value: "b", Enabled: true}}
	r.PathParams = []KeyValue{{Name: "id", Value: "7", Enabled: true}}
	r.Options.FollowRedirects = false
	r.Options.AttachCookies = false
	r.Options.ProxyURL = "http://proxy:8080"
	got := Normalize(r)

	defaults := NewRequest()
	if got.Method != defaults.Method || !reflect.DeepEqual(got.Body, defaults.Body) || got.Query != nil || got.PathParams != nil {
		t.Errorf("Normalize kept HTTP fields: method %q, body %+v, query %v, path %v", got.Method, got.Body, got.Query, got.PathParams)
	}
	if got.Options.FollowRedirects != defaults.Options.FollowRedirects || got.Options.AttachCookies != defaults.Options.AttachCookies || got.Options.ProxyURL != "" {
		t.Errorf("Normalize kept HTTP-only options: %+v", got.Options)
	}
	want := GRPCKind.Example()
	if !reflect.DeepEqual(got.Headers, want.Headers) || got.Auth != want.Auth || got.Options.VerifySSL != want.Options.VerifySSL ||
		got.Options.TimeoutSeconds != want.Options.TimeoutSeconds || !reflect.DeepEqual(got.Payload, want.Payload) {
		t.Errorf("Normalize changed fields gRPC uses:\n got %+v\nwant %+v", got, want)
	}
}

func TestDigestAuthIsDroppedOnlyFromGRPC(t *testing.T) {
	digest := Auth{Type: AuthDigest, Username: "ada", Password: "secret"}
	for _, k := range Kinds {
		r := k.Example()
		r.Auth = digest
		want := digest
		if k == GRPCKind {
			want = Auth{Type: AuthNone}
		}
		if got := Normalize(r).Auth; got != want {
			t.Errorf("%s: Normalize made digest auth %+v, want %+v", k.Label, got, want)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var loaded Request
		if err := json.Unmarshal(data, &loaded); err != nil || loaded.Auth != want {
			t.Errorf("%s: a history entry with digest auth loaded with %+v, %v; want %+v", k.Label, loaded.Auth, err, want)
		}
	}
}

func TestNormalizeKeepsHTTPOnlyOptionsForGraphQL(t *testing.T) {
	r := GraphQLKind.Example()
	r.Options.AttachCookies = false
	r.Options.ProxyURL = "http://proxy:8080"
	if got := Normalize(r); !reflect.DeepEqual(got.Options, r.Options) {
		t.Fatalf("GraphQL is sent over HTTP, so its options stay: %+v", got.Options)
	}
}

func TestResolveGRPC(t *testing.T) {
	r := GRPCKind.New()
	r.URL = "${HOST}:50051"
	r.Payload = GRPC{
		Method:  "acme.${SVC}/Get",
		Message: `{"id": "${ID}", "raw": "$ID"}`,
		Protos:  ProtoSet{Files: []string{"${DIR}/a.proto"}},
	}
	env := MapLookup(map[string]string{"HOST": "localhost", "ID": "7", "SVC": "x", "DIR": "d"})
	resolved, err := Resolve(r, env)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.URL != "localhost:50051" {
		t.Errorf("URL = %q: a bare gRPC address must not gain http://", resolved.URL)
	}
	want := GRPC{Method: "acme.${SVC}/Get", Message: `{"id": "7", "raw": "7"}`, Protos: ProtoSet{Files: []string{"${DIR}/a.proto"}}}
	if got := resolved.Payload.(GRPC); !reflect.DeepEqual(got, want) {
		t.Errorf("resolved payload = %+v\nwant %+v", got, want)
	}

	resolved.Payload.(GRPC).Protos.Files[0] = "changed"
	if r.Payload.(GRPC).Protos.Files[0] != "${DIR}/a.proto" {
		t.Error("the resolved request shares its proto paths with the original")
	}

	r.Options.SubstituteBodyVariables = false
	resolved, _ = Resolve(r, env)
	if got := resolved.Payload.(GRPC).Message; got != r.Payload.(GRPC).Message {
		t.Errorf("with body substitution off the message is sent as written, got %q", got)
	}
}

func TestResolveStillAddsASchemeForHTTP(t *testing.T) {
	r := NewRequest()
	r.URL = "localhost:8000/users"
	resolved, err := Resolve(r, MapLookup(nil))
	if err != nil || resolved.URL != "http://localhost:8000/users" {
		t.Fatalf("Resolve = %q, %v", resolved.URL, err)
	}
}

func TestParseGRPCTarget(t *testing.T) {
	for _, c := range []struct {
		in   string
		want GRPCTarget
	}{
		{"localhost:50051", GRPCTarget{"localhost:50051", false}},
		{"LOCALHOST:50051", GRPCTarget{"LOCALHOST:50051", false}},
		{"127.0.0.1:9000", GRPCTarget{"127.0.0.1:9000", false}},
		{"127.4.5.6:9000", GRPCTarget{"127.4.5.6:9000", false}},
		{"[::1]:9000", GRPCTarget{"[::1]:9000", false}},
		{"0.0.0.0:50051", GRPCTarget{"0.0.0.0:50051", false}},
		{"[::]:50051", GRPCTarget{"[::]:50051", false}},
		{"localhost", GRPCTarget{"localhost:80", false}},
		{"api.example.com:8443", GRPCTarget{"api.example.com:8443", true}},
		{"api.example.com", GRPCTarget{"api.example.com:443", true}},
		{"10.0.0.5:50051", GRPCTarget{"10.0.0.5:50051", true}},
		{"grpc://api.example.com:50051", GRPCTarget{"api.example.com:50051", false}},
		{"http://api.example.com", GRPCTarget{"api.example.com:80", false}},
		{"grpcs://localhost:50051", GRPCTarget{"localhost:50051", true}},
		{"HTTPS://localhost", GRPCTarget{"localhost:443", true}},
		{"grpc://localhost:50051/", GRPCTarget{"localhost:50051", false}},
		{"  localhost:50051  ", GRPCTarget{"localhost:50051", false}},
	} {
		got, err := ParseGRPCTarget(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseGRPCTarget(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	for in, wantInError := range map[string]string{
		"":                                  "address",
		"ftp://example.com":                 "ftp",
		"localhost:50051/acme.Users/Get":    "Method",
		"grpc://localhost:50051/acme.Users": "Method",
		"localhost:http":                    "port",
		"localhost:70000":                   "port",
		"grpc://:50051":                     "host",
	} {
		if got, err := ParseGRPCTarget(in); err == nil || !strings.Contains(err.Error(), wantInError) {
			t.Errorf("ParseGRPCTarget(%q) = %+v, %v; want an error naming %q", in, got, err, wantInError)
		}
	}
}

func TestStatusOfGRPC(t *testing.T) {
	r := GRPCKind.New()
	for _, c := range []struct {
		name string
		resp Response
		want Status
	}{
		{"ok", Response{GRPC: &GRPCStatus{Code: 0}}, Status{"OK", "", StatusClassSuccess}},
		{"not found", Response{GRPC: &GRPCStatus{Code: 5, Message: "no book 42"}}, Status{"NOT_FOUND", "no book 42", StatusClassError}},
		{"deadline", Response{GRPC: &GRPCStatus{Code: 4, Message: "context deadline exceeded"}}, Status{"DEADLINE_EXCEEDED", "context deadline exceeded", StatusClassError}},
		{"unauthenticated", Response{GRPC: &GRPCStatus{Code: 16}}, Status{"UNAUTHENTICATED", "", StatusClassError}},
		{"non-standard code", Response{GRPC: &GRPCStatus{Code: 42, Message: "custom"}}, Status{"CODE(42)", "custom", StatusClassError}},
		{"http status ignored", Response{StatusCode: 200, Reason: "OK", GRPC: &GRPCStatus{Code: 13, Message: "boom"}}, Status{"INTERNAL", "boom", StatusClassError}},
		{"no status", Response{}, Status{"UNKNOWN", "no gRPC status", StatusClassError}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := StatusOf(r, &c.resp); got != c.want {
				t.Fatalf("StatusOf = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestDisplayNameFallsBackToThePayloadsLabel(t *testing.T) {
	grpc := GRPCKind.New()
	grpc.URL = "localhost:50051"
	gql := GraphQLKind.New()
	gql.URL = "http://api.test/graphql"
	for _, c := range []struct {
		req     Request
		payload Payload
		want    string
	}{
		{grpc, GRPC{Method: "library.v1.Library/GetBook"}, "Library/GetBook"},
		{grpc, GRPC{Method: "/Library/GetBook"}, "Library/GetBook"},
		{grpc, GRPC{Method: "half.typed"}, "half.typed"},
		{grpc, GRPC{}, "localhost:50051"},
		{gql, GraphQL{OperationName: "User"}, "User"},
		{gql, GraphQL{Query: "{ a }"}, "http://api.test/graphql"},
	} {
		c.req.Payload = c.payload
		if got := c.req.DisplayName(); got != c.want {
			t.Errorf("DisplayName of %+v = %q, want %q", c.payload, got, c.want)
		}
		c.req.Name = "Named"
		if got := c.req.DisplayName(); got != "Named" {
			t.Errorf("a name comes before the label, got %q", got)
		}
	}
}

func TestResponseContentTypePrefersTheDecodedBodysType(t *testing.T) {
	resp := Response{Headers: []Header{{Name: "Content-Type", Value: "application/grpc"}}}
	if got := resp.ContentType(); got != "application/grpc" {
		t.Fatalf("ContentType = %q", got)
	}
	resp.BodyContentType = "application/json"
	if got := resp.ContentType(); got != "application/json" {
		t.Fatalf("ContentType = %q, want the decoded body's type", got)
	}
}

func TestGRPCResponseJSONRoundTrip(t *testing.T) {
	want := &Response{
		Proto:           "gRPC",
		Headers:         []Header{{Name: "x-a", Value: "1"}},
		Trailers:        []Header{{Name: "grpc-status", Value: "5"}},
		Body:            []byte(`{"code": "NOT_FOUND"}`),
		BodyContentType: "application/json",
		GRPC:            &GRPCStatus{Code: 5, Message: "gone"},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Response
	if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(&got, want) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	data, _ = json.Marshal(Response{StatusCode: 200})
	for _, key := range []string{"Trailers", "BodyContentType", "GRPC"} {
		if strings.Contains(string(data), key) {
			t.Errorf("an HTTP response's history JSON gained %s: %s", key, data)
		}
	}
}

func TestGRPCPayloadSize(t *testing.T) {
	r := GRPCKind.New()
	r.Payload = GRPC{Method: "a/b", Message: "{}", Protos: ProtoSet{Files: []string{"x.proto"}, ImportPaths: []string{"p"}}}
	if got := r.PayloadSize(); got != 3+2+7+1 {
		t.Fatalf("PayloadSize = %d", got)
	}

	files := make([]string, 1, 2)
	files[0] = "x.proto"
	r.Payload = GRPC{Protos: ProtoSet{Files: files, ImportPaths: []string{"p"}}}
	r.PayloadSize()
	if spare := files[:2][1]; spare != "" {
		t.Fatalf("PayloadSize wrote %q into the files' spare capacity", spare)
	}
}
