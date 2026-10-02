package client

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/darrenburns/posting/internal/model"
)

func grpcRequest(addr, method, message string) model.Request {
	r := model.GRPCKind.New()
	r.URL = addr
	r.Payload = model.GRPC{Method: method, Message: message}
	return r
}

func callGRPC(t *testing.T, req model.Request) (*model.Response, error) {
	t.Helper()
	return NewGRPC("posting-test", TLSSettings{}, "testdata").Send(context.Background(), Call{Request: req})
}

func mustCallGRPC(t *testing.T, req model.Request) *model.Response {
	t.Helper()
	resp, err := callGRPC(t, req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// assertJSON compares JSON by value, so formatting doesn't matter.
func assertJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("body isn't JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("body =\n%s\nwant\n%s", got, want)
	}
}

func headerValues(headers []model.Header, name string) []string {
	var values []string
	for _, h := range headers {
		if h.Name == name {
			values = append(values, h.Value)
		}
	}
	return values
}

func TestGRPCUnaryCall(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "${ISBN}"}`)
	resp, err := NewGRPC("posting-test", TLSSettings{}, "").Send(context.Background(), Call{Request: req, Variables: map[string]string{"ISBN": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if status := model.StatusOf(req, resp); status != (model.Status{Code: "OK", Class: model.StatusClassSuccess}) {
		t.Fatalf("status = %+v", status)
	}
	if resp.StatusCode != 0 || resp.Proto != "gRPC" || resp.ContentType() != "application/json" {
		t.Errorf("StatusCode %d, Proto %q, ContentType %q", resp.StatusCode, resp.Proto, resp.ContentType())
	}
	assertJSON(t, resp.Body, `{
		"isbn": "2", "title": "Dune Messiah", "author": {"name": "Frank Herbert", "born": 1920},
		"genre": "GENRE_FICTION", "tags": [], "labels": {}, "editors": {}
	}`)
	if !strings.Contains(string(resp.Body), "\n  \"isbn\"") {
		t.Errorf("the body is indented, as a server would rarely send it:\n%s", resp.Body)
	}
	if got := headerValues(resp.Trailers, "x-books-served"); !reflect.DeepEqual(got, []string{"1"}) {
		t.Errorf("trailers = %+v", resp.Trailers)
	}
	if got := headerValues(resp.Trailers, "grpc-status"); !reflect.DeepEqual(got, []string{"0"}) {
		t.Errorf("trailers = %+v; want grpc-status 0 among them", resp.Trailers)
	}
	if resp.URL != addr || resp.Elapsed <= 0 || resp.ReceivedAt.IsZero() {
		t.Errorf("URL %q, elapsed %v, received %v", resp.URL, resp.Elapsed, resp.ReceivedAt)
	}
}

func TestGRPCErrorStatusIsAResponse(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "404"}`)
	resp := mustCallGRPC(t, req)
	if status := model.StatusOf(req, resp); status != (model.Status{Code: "NOT_FOUND", Text: `no book "404"`, Class: model.StatusClassError}) {
		t.Fatalf("status = %+v", status)
	}
	assertJSON(t, resp.Body, `{"code": "NOT_FOUND", "message": "no book \"404\""}`)
	want := []model.Header{{Name: "grpc-status", Value: "5"}, {Name: "grpc-message", Value: `no book "404"`}}
	if !reflect.DeepEqual(resp.Trailers, want) {
		t.Fatalf("trailers = %+v, want %+v", resp.Trailers, want)
	}
}

func TestGRPCErrorDetailsAreDecodedWhereKnown(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	resp := mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "detailed"}`))
	assertJSON(t, resp.Body, `{
		"code": "NOT_FOUND", "message": "no book detailed",
		"details": [
			{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "BOOK_MISSING", "domain": "library.test", "metadata": {}},
			{"@type": "type.googleapis.com/acme.Secret", "value": "AQID"}
		]
	}`)
}

func TestGRPCServerStreamIsAnArray(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	resp := mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "Frank Herbert"}`))
	var books []map[string]any
	if err := json.Unmarshal(resp.Body, &books); err != nil {
		t.Fatalf("%v\n%s", err, resp.Body)
	}
	if len(books) != 2 || books[0]["title"] != "Dune" || books[1]["title"] != "Dune Messiah" {
		t.Fatalf("books = %v", books)
	}
	if got := headerValues(resp.Trailers, "x-books-served"); !reflect.DeepEqual(got, []string{"2"}) {
		t.Errorf("trailers = %+v", resp.Trailers)
	}

	resp = mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "Ursula K. Le Guin"}`))
	assertJSON(t, resp.Body, `[]`)
}

func TestGRPCEndlessStreamIsTruncatedAtTheResponseCap(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	g := NewGRPC("posting-test", TLSSettings{}, "")
	g.maxResponse = 10 << 10
	req := grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "forever"}`)
	req.Options.TimeoutSeconds = 5
	started := time.Now()
	resp, err := g.Send(context.Background(), Call{Request: req})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("the call ran %v; it should stop at the cap, not the deadline", elapsed)
	}
	if status := model.StatusOf(req, resp); status.Code != "CANCELLED" || status.Text != "response truncated at 10.00 KB" {
		t.Errorf("status = %+v, want one saying the response was truncated", status)
	}
	var books []map[string]any
	if err := json.Unmarshal(resp.Body, &books); err != nil {
		t.Fatal(err)
	}
	if len(books) == 0 || books[0]["title"] != "Dune" {
		t.Fatalf("the messages before the cap are kept: %d books", len(books))
	}
	_, svc := librarySchema(t)
	book := dynamicpb.NewMessage(svc.Methods().ByName("ListBooks").Output())
	if err := protojson.Unmarshal([]byte(libraryBooks[0]), book); err != nil {
		t.Fatal(err)
	}
	if most := g.maxResponse / proto.Size(book); len(books) > most {
		t.Errorf("kept %d books, more than the %d that fit under the cap", len(books), most)
	}
}

func TestGRPCStreamThatFailsWithoutMessagesShowsItsStatus(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "nobody"}`)
	resp := mustCallGRPC(t, req)
	assertJSON(t, resp.Body, `{"code": "NOT_FOUND", "message": "no books by nobody"}`)
}

func TestGRPCClientStreamSendsEachArrayElement(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	resp := mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/ShelveBooks", `[{"title": "Dune"}, {"title": "Emma"}]`))
	assertJSON(t, resp.Body, `{"shelved": 2, "titles": ["Dune", "Emma"]}`)

	resp = mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/ShelveBooks", `{"title": "Alone"}`))
	assertJSON(t, resp.Body, `{"shelved": 1, "titles": ["Alone"]}`)
}

func TestGRPCBidiStream(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	resp := mustCallGRPC(t, grpcRequest(addr, "library.v1.Library/Chat", `[{"text": "hi"}, {"text": "there"}]`))
	assertJSON(t, resp.Body, `[{"text": "HI"}, {"text": "THERE"}]`)
}

// An echo server blocks once its replies fill the flow-control window, so a
// client that sends everything before reading never finishes sending.
func TestGRPCBidiStreamLargerThanTheFlowControlWindow(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	const count, size = 200, 8 << 10
	text := strings.Repeat("a", size)
	messages := make([]string, count)
	for i := range messages {
		messages[i] = `{"text": "` + text + `"}`
	}
	req := grpcRequest(addr, "library.v1.Library/Chat", "["+strings.Join(messages, ",")+"]")
	req.Options.TimeoutSeconds = 5
	resp := mustCallGRPC(t, req)
	if status := model.StatusOf(req, resp); status.Code != "OK" {
		t.Fatalf("status = %+v", status)
	}
	var echoes []struct{ Text string }
	if err := json.Unmarshal(resp.Body, &echoes); err != nil {
		t.Fatal(err)
	}
	if len(echoes) != count {
		t.Fatalf("got %d echoes, want %d", len(echoes), count)
	}
	for i, echo := range echoes {
		if echo.Text != strings.ToUpper(text) {
			t.Fatalf("echo %d is %d bytes of %.10q…", i, len(echo.Text), echo.Text)
		}
	}
}

func TestGRPCMetadataGoesOutAndHeadersComeBack(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "1"}`)
	req.Headers = []model.KeyValue{
		{Name: "X-Tenant", Value: "${TENANT}", Enabled: true},
		{Name: "x-tenant", Value: "second", Enabled: true},
		{Name: "x-off", Value: "no", Enabled: false},
	}
	req.Auth = model.Auth{Type: model.AuthBearer, Token: "${TOKEN}"}
	resp, err := NewGRPC("posting-test", TLSSettings{}, "").Send(context.Background(), Call{Request: req, Variables: map[string]string{"TENANT": "core", "TOKEN": "t0k"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := headerValues(resp.Headers, "echo-x-tenant"); !reflect.DeepEqual(got, []string{"core", "second"}) {
		t.Errorf("echo-x-tenant = %v; headers %+v", got, resp.Headers)
	}
	if got := headerValues(resp.Headers, "echo-authorization"); !reflect.DeepEqual(got, []string{"Bearer t0k"}) {
		t.Errorf("echo-authorization = %v", got)
	}
	if got := headerValues(resp.Headers, "echo-x-off"); got != nil {
		t.Errorf("a disabled header was sent: %v", got)
	}
	if got := headerValues(resp.Headers, "x-library-branch"); !reflect.DeepEqual(got, []string{"central"}) {
		t.Errorf("response headers = %+v", resp.Headers)
	}

	req.Auth = model.Auth{Type: model.AuthBasic, Username: "ada", Password: "pw"}
	resp = mustCallGRPC(t, req)
	if got := headerValues(resp.Headers, "echo-authorization"); !reflect.DeepEqual(got, []string{"Basic YWRhOnB3"}) {
		t.Errorf("basic auth sent as %v", got)
	}
}

func TestGRPCDeadlineKeepsTheMessagesThatArrived(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "stall"}`)
	req.Options.TimeoutSeconds = 0.5
	resp := mustCallGRPC(t, req)
	if status := model.StatusOf(req, resp); status.Code != "DEADLINE_EXCEEDED" || status.Class != model.StatusClassError {
		t.Fatalf("status = %+v", status)
	}
	var books []map[string]any
	if err := json.Unmarshal(resp.Body, &books); err != nil || len(books) != 2 {
		t.Fatalf("body = %s, %v; want the two books sent before the deadline", resp.Body, err)
	}
}

func TestGRPCTimeoutBeforeTheServerAnswersIsAnError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	go func() {
		// Accept and say nothing, so the call can't start.
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
		}
	}()
	req := grpcRequest(lis.Addr().String(), "library.v1.Library/GetBook", `{}`)
	req.Payload = model.GRPC{Method: "library.v1.Library/GetBook", Protos: model.ProtoSet{Files: []string{"library.proto"}}}
	req.Options.TimeoutSeconds = 0.3
	_, err = callGRPC(t, req)
	if err == nil || err.Error() != "request timed out after 0.3s" {
		t.Fatalf("err = %v", err)
	}
}

func TestGRPCCallTheServerNeverAnswersIsAnError(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "hang"}`)
	req.Options.TimeoutSeconds = 0.3
	resp, err := callGRPC(t, req)
	if err == nil || err.Error() != "request timed out after 0.3s" {
		t.Fatalf("Send = %+v, %v; a status grpc-go made up isn't the server's response", resp, err)
	}
}

func TestGRPCConnectionRefusedIsAnError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close()
	var mu sync.Mutex
	var trace []model.TraceEvent
	resp, err := NewGRPC("posting-test", TLSSettings{}, "").Send(context.Background(), Call{
		Request: grpcRequest(addr, "library.v1.Library/GetBook", `{}`),
		OnTrace: func(e model.TraceEvent) {
			mu.Lock()
			trace = append(trace, e)
			mu.Unlock()
		},
	})
	if err == nil || !strings.Contains(err.Error(), "couldn't connect to "+addr) || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("Send = %+v, %v", resp, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if last := trace[len(trace)-1]; last != (model.TraceEvent{Stage: model.TraceConnect, State: model.TraceFailed}) {
		t.Fatalf("trace ends %+v", last)
	}
}

func TestGRPCDiscoveryWithoutReflection(t *testing.T) {
	addr := startLibrary(t, libraryOptions{noReflection: true})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "1"}`)
	_, err := NewGRPC("posting-test", TLSSettings{}, "").Describe(context.Background(), Call{Request: req})
	if !errors.Is(err, ErrNoReflection) || !strings.Contains(err.Error(), "Proto tab") {
		t.Fatalf("Describe err = %v, want the no-reflection hint", err)
	}
	if _, err := callGRPC(t, req); !errors.Is(err, ErrNoReflection) {
		t.Fatalf("Send err = %v, want the no-reflection hint", err)
	}

	req.Payload = model.GRPC{Method: "library.v1.Library/GetBook", Message: `{"isbn": "1"}`, Protos: model.ProtoSet{Files: []string{"library.proto"}}}
	resp := mustCallGRPC(t, req)
	if !strings.Contains(string(resp.Body), `"Dune"`) {
		t.Fatalf("with proto files the call works without reflection: %s", resp.Body)
	}
}

// A server without reflection can end the reflection call before Posting
// sends its first request. Waiting for the server's headers first makes that
// order certain, as a busy machine sometimes does.
func TestGRPCReflectionEndedBeforeTheFirstRequestIsNoReflection(t *testing.T) {
	addr := startLibrary(t, libraryOptions{noReflection: true})
	conn, err := grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			stream, err := streamer(ctx, desc, cc, method, opts...)
			if err == nil {
				_, _ = stream.Header()
			}
			return stream, err
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := reflectFiles(context.Background(), conn, nil); !errors.Is(err, ErrNoReflection) {
		t.Fatalf("reflectFiles err = %v, want the no-reflection hint", err)
	}
}

func TestGRPCDescribeByReflection(t *testing.T) {
	for name, opts := range map[string]libraryOptions{"v1": {}, "v1alpha": {v1alphaOnly: true}} {
		t.Run(name, func(t *testing.T) {
			addr := startLibrary(t, opts)
			schema, err := NewGRPC("posting-test", TLSSettings{}, "").Describe(context.Background(), Call{Request: grpcRequest(addr, "", "")})
			if err != nil {
				t.Fatal(err)
			}
			want := []Method{
				{Name: "library.v1.Library/Chat", Streaming: BidiStream, Input: "library.v1.ChatMessage", Output: "library.v1.ChatMessage"},
				{Name: "library.v1.Library/GetBook", Streaming: Unary, Input: "library.v1.GetBookRequest", Output: "library.v1.Book"},
				{Name: "library.v1.Library/ListBooks", Streaming: ServerStream, Input: "library.v1.ListBooksRequest", Output: "library.v1.Book"},
				{Name: "library.v1.Library/ShelveBooks", Streaming: ClientStream, Input: "library.v1.Book", Output: "library.v1.ShelveSummary"},
			}
			for i := range schema.Methods {
				schema.Methods[i].Template = ""
			}
			if !reflect.DeepEqual(schema.Methods, want) {
				t.Fatalf("methods = %+v\nwant %+v", schema.Methods, want)
			}

			resp := mustCallGRPC(t, grpcRequest(addr, "library.v1.Library.GetBook", `{"isbn": "3"}`))
			if !strings.Contains(string(resp.Body), "Hawking") {
				t.Fatalf("the dotted method form works too: %s", resp.Body)
			}
		})
	}
}

func TestGRPCDescribeProtoFilesNeverDials(t *testing.T) {
	for _, url := range []string{"unreachable.invalid:1", ""} {
		req := grpcRequest(url, "", "")
		req.Payload = model.GRPC{Protos: model.ProtoSet{Files: []string{"library.proto"}}}
		schema, err := NewGRPC("posting-test", TLSSettings{}, "testdata").Describe(context.Background(), Call{Request: req})
		if err != nil || len(schema.Methods) != 4 {
			t.Fatalf("Describe with address %q = %+v, %v", url, schema, err)
		}
	}
}

func TestGRPCDescribeProtoFilesIgnoresTheAddressAndMetadata(t *testing.T) {
	for name, edit := range map[string]func(*model.Request){
		"undefined address variable": func(r *model.Request) { r.URL = "${HOST}" },
		"digest auth":                func(r *model.Request) { r.Auth = model.Auth{Type: model.AuthDigest, Username: "ada"} },
		"reserved metadata": func(r *model.Request) {
			r.Headers = []model.KeyValue{{Name: "grpc-timeout", Value: "1S", Enabled: true}}
		},
	} {
		req := grpcRequest("localhost:1", "", "")
		req.Payload = model.GRPC{Protos: model.ProtoSet{Files: []string{"library.proto"}}}
		edit(&req)
		schema, err := NewGRPC("posting-test", TLSSettings{}, "testdata").Describe(context.Background(), Call{Request: req})
		if err != nil || len(schema.Methods) != 4 {
			t.Errorf("%s: Describe = %d methods, %v; proto files need neither", name, len(schema.Methods), err)
		}
	}
}

func TestGRPCDescriptorSetFiles(t *testing.T) {
	files, _ := librarySchema(t)
	set := &descriptorpb.FileDescriptorSet{}
	for _, path := range []string{"google/protobuf/timestamp.proto", "library.proto"} {
		fd, err := files.FindFileByPath(path)
		if err != nil {
			t.Fatal(err)
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(fd))
	}
	data, _ := proto.Marshal(set)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.protoset"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	addr := startLibrary(t, libraryOptions{noReflection: true})
	req := grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "1"}`)
	req.Payload = model.GRPC{Method: "library.v1.Library/GetBook", Message: `{"isbn": "1"}`, Protos: model.ProtoSet{Files: []string{"library.protoset"}}}
	resp, err := NewGRPC("posting-test", TLSSettings{}, dir).Send(context.Background(), Call{Request: req})
	if err != nil || !strings.Contains(string(resp.Body), `"Dune"`) {
		t.Fatalf("Send = %+v, %v", resp, err)
	}
}

func TestGRPCProtoFilesAreRecompiledWhenEdited(t *testing.T) {
	dir := t.TempDir()
	proto := filepath.Join(dir, "protos", "svc.proto")
	if err := os.MkdirAll(filepath.Dir(proto), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(methods string) {
		t.Helper()
		source := "syntax = \"proto3\";\npackage svc;\nmessage M {}\nservice S {\n" + methods + "}\n"
		if err := os.WriteFile(proto, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	client := NewGRPC("posting-test", TLSSettings{}, dir)
	req := grpcRequest("localhost:1", "", "")
	req.Payload = model.GRPC{Protos: model.ProtoSet{Files: []string{"protos/svc.proto"}, ImportPaths: []string{"protos"}}}
	describe := func() []string {
		t.Helper()
		schema, err := client.Describe(context.Background(), Call{Request: req})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, m := range schema.Methods {
			names = append(names, m.Name)
		}
		return names
	}
	write("  rpc A(M) returns (M);\n")
	if got := describe(); !reflect.DeepEqual(got, []string{"svc.S/A"}) {
		t.Fatalf("methods = %v", got)
	}
	write("  rpc A(M) returns (M);\n  rpc B(M) returns (M);\n")
	if got := describe(); !reflect.DeepEqual(got, []string{"svc.S/A", "svc.S/B"}) {
		t.Fatalf("after an edit, methods = %v", got)
	}
}

func TestGRPCProtoFileErrors(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"broken.proto": "syntax = \"proto3\";\nmessage {",
		"text.pb":      "syntax = \"proto3\";",
		"ok.proto":     "syntax = \"proto3\";\nimport \"missing.proto\";",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name        string
		protos      model.ProtoSet
		wantInError string
	}{
		{"missing file", model.ProtoSet{Files: []string{"nope.proto"}}, "nope.proto"},
		{"outside the import paths", model.ProtoSet{Files: []string{"ok.proto"}, ImportPaths: []string{"elsewhere"}}, "import paths"},
		{"syntax error", model.ProtoSet{Files: []string{"broken.proto"}}, "broken.proto:2"},
		{"missing import", model.ProtoSet{Files: []string{"ok.proto"}}, "missing.proto"},
		{"not a descriptor set", model.ProtoSet{Files: []string{"text.pb"}}, "text.pb isn't a descriptor set"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := grpcRequest("localhost:1", "", "")
			req.Payload = model.GRPC{Protos: c.protos}
			_, err := NewGRPC("posting-test", TLSSettings{}, root).Describe(context.Background(), Call{Request: req})
			if err == nil || !strings.Contains(err.Error(), c.wantInError) {
				t.Fatalf("err = %v, want one naming %q", err, c.wantInError)
			}
		})
	}
}

func TestGRPCMethodAndMessageErrorsComeBeforeDialing(t *testing.T) {
	for _, c := range []struct{ method, message, wantInError string }{
		{"", "{}", "choose a method"},
		{"GetBook", "{}", "package.Service/Method"},
		{"library.v1.Shelf/GetBook", "{}", "services: library.v1.Library"},
		{"library.v1.Library/GetBok", "{}", "methods: GetBook, ListBooks"},
		{"library.v1.Library/GetBook", `[{"isbn": "1"}]`, "GetBook is a unary method and takes one message, not an array"},
		{"library.v1.Library/ListBooks", `[{}]`, "ListBooks is a server stream method"},
		{"library.v1.Library/GetBook", `{"isbn": 1`, "isn't valid JSON"},
		{"library.v1.Library/ShelveBooks", `{"title": "a"} {"title": "b"}`, "in an array"},
		{"library.v1.Library/GetBook", `{"isbm": "1"}`, "isn't a library.v1.GetBookRequest"},
		{"library.v1.Library/ShelveBooks", `[{"title": "a"}, {"titel": "b"}]`, "message 2"},
	} {
		req := grpcRequest("unreachable.invalid:1", c.method, c.message)
		req.Payload = model.GRPC{Method: c.method, Message: c.message, Protos: model.ProtoSet{Files: []string{"library.proto"}}}
		_, err := callGRPC(t, req)
		if err == nil || !strings.Contains(err.Error(), c.wantInError) {
			t.Errorf("%s %s: err = %v, want one naming %q", c.method, c.message, err, c.wantInError)
		}
	}
}

func TestGRPCTraceCoversTheCall(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	var mu sync.Mutex
	var reported []model.TraceEvent
	resp, err := NewGRPC("posting-test", TLSSettings{}, "").Send(context.Background(), Call{
		Request: grpcRequest(addr, "library.v1.Library/GetBook", `{"isbn": "1"}`),
		OnTrace: func(e model.TraceEvent) {
			mu.Lock()
			reported = append(reported, e)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stages []model.TraceStage
	for _, e := range resp.Trace {
		stages = append(stages, e.Stage)
		want := model.TraceComplete
		if e.Stage == model.TraceTLS {
			want = model.TraceSkipped
		}
		if e.State != want {
			t.Errorf("%s is %v, want %v", e.Stage, e.State, want)
		}
	}
	if !reflect.DeepEqual(stages, model.TraceStages) {
		t.Fatalf("trace stages = %v", stages)
	}
	if len(reported) == 0 || reported[len(reported)-1] != resp.Trace[len(resp.Trace)-1] {
		t.Fatalf("OnTrace didn't report the call as it went: %+v", reported)
	}
}

func TestGRPCTLSTargets(t *testing.T) {
	tlsAddr := startLibrary(t, libraryOptions{tls: true})
	plainAddr := startLibrary(t, libraryOptions{})
	_, tlsPort, _ := net.SplitHostPort(tlsAddr)
	send := func(url string, verify bool) (*model.Response, error) {
		req := grpcRequest(url, "library.v1.Library/GetBook", `{"isbn": "1"}`)
		req.Options.VerifySSL = verify
		return callGRPC(t, req)
	}

	resp, err := send("grpcs://"+tlsAddr, false)
	if err != nil {
		t.Fatalf("grpcs:// with verification off: %v", err)
	}
	if tls := resp.Trace[1]; tls.Stage != model.TraceTLS || tls.State != model.TraceComplete {
		t.Errorf("a TLS call's trace has the handshake: %+v", resp.Trace)
	}
	if _, err := send("https://localhost:"+tlsPort, false); err != nil {
		t.Errorf("https:// is TLS too: %v", err)
	}
	if _, err := send("grpcs://"+tlsAddr, true); err == nil || !strings.Contains(err.Error(), "certificate verification failed") {
		t.Errorf("a self-signed certificate must fail verification, got %v", err)
	}
	if _, err := send("grpc://"+tlsAddr, false); err == nil || !strings.Contains(err.Error(), "grpcs://"+tlsAddr) {
		t.Errorf("plaintext to a TLS server should suggest grpcs://, got %v", err)
	}
	if _, err := send(tlsAddr, false); err == nil {
		t.Error("a bare loopback address is plaintext, so it can't reach a TLS server")
	}
	if _, err := send("grpcs://"+plainAddr, false); err == nil || !strings.Contains(err.Error(), "grpc://"+plainAddr) {
		t.Errorf("TLS to a plaintext server should suggest grpc://, got %v", err)
	}
	if _, err := send(plainAddr, true); err != nil {
		t.Errorf("a bare loopback address is plaintext: %v", err)
	}
}

func TestGRPCLoadsTheCABundleOnlyForTLSAndWhenItIsFixed(t *testing.T) {
	cert := selfSigned(t)
	plainAddr := startLibrary(t, libraryOptions{})
	tlsAddr := startLibrary(t, libraryOptions{tls: true, cert: cert})
	bundle := filepath.Join(t.TempDir(), "ca.pem")
	g := NewGRPC("posting-test", TLSSettings{CABundle: bundle}, "testdata")
	send := func(url string) (*model.Response, error) {
		req := grpcRequest(url, "library.v1.Library/GetBook", `{"isbn": "1"}`)
		req.Options.VerifySSL = true
		return g.Send(context.Background(), Call{Request: req})
	}

	if _, err := send(plainAddr); err != nil {
		t.Fatalf("a plaintext call has no use for the CA bundle: %v", err)
	}
	protos := grpcRequest(plainAddr, "", "")
	protos.Payload = model.GRPC{Protos: model.ProtoSet{Files: []string{"library.proto"}}}
	if _, err := g.Describe(context.Background(), Call{Request: protos}); err != nil {
		t.Fatalf("listing proto files has no use for the CA bundle: %v", err)
	}
	if _, err := send("grpcs://" + tlsAddr); err == nil || !strings.Contains(err.Error(), "CA bundle") {
		t.Fatalf("a TLS call with a missing CA bundle: %v", err)
	}

	if err := os.WriteFile(bundle, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := send("grpcs://" + tlsAddr); err != nil {
		t.Fatalf("once the CA bundle is fixed, the server's certificate is trusted: %v", err)
	}
}

func TestOutgoingMetadata(t *testing.T) {
	md, err := outgoingMetadata([]model.KeyValue{
		{Name: " X-Trace-Bin ", Value: "AQID", Enabled: true},
		{Name: "x-raw-bin", Value: "AQI", Enabled: true},
		{Name: "Authorization", Value: "replaced", Enabled: true},
		{Name: "", Value: "blank names are skipped", Enabled: true},
	}, model.Auth{Type: model.AuthBearer, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"x-trace-bin": {"\x01\x02\x03"}, "x-raw-bin": {"\x01\x02"}, "authorization": {"Bearer t"}}
	if !reflect.DeepEqual(map[string][]string(md), want) {
		t.Fatalf("metadata = %q", md)
	}

	for _, c := range []struct {
		header      model.KeyValue
		auth        model.Auth
		wantInError string
	}{
		{model.KeyValue{Name: "x-bad-bin", Value: "!!", Enabled: true}, model.Auth{}, "x-bad-bin"},
		{model.KeyValue{Name: "Content-Type", Value: "x", Enabled: true}, model.Auth{}, "content-type"},
		{model.KeyValue{Name: "grpc-timeout", Value: "1S", Enabled: true}, model.Auth{}, "grpc-timeout"},
		{model.KeyValue{Name: ":authority", Value: "x", Enabled: true}, model.Auth{}, ":authority"},
		{model.KeyValue{Name: "x tenant", Value: "x", Enabled: true}, model.Auth{}, "x tenant"},
	} {
		if _, err := outgoingMetadata([]model.KeyValue{c.header}, c.auth); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.wantInError)) {
			t.Errorf("%+v %+v: err = %v, want one naming %q", c.header, c.auth, err, c.wantInError)
		}
	}
}

func TestGRPCTemplates(t *testing.T) {
	files, svc := librarySchema(t)
	got := map[string]string{}
	methods := svc.Methods()
	for i := range methods.Len() {
		m := methodOf(methods.Get(i))
		got[string(methods.Get(i).Name())] = m.Template
	}
	book := `{
  "isbn": "",
  "title": "",
  "author": {
    "name": "",
    "born": 0
  },
  "genre": "GENRE_FICTION",
  "tags": [
    ""
  ],
  "published": "1970-01-01T00:00:00Z",
  "labels": {
    "": ""
  },
  "hardback": false,
  "note": "",
  "shelf": {
    "name": "",
    "parent": {
      "name": "",
      "parent": {}
    }
  },
  "editors": {
    "0": {
      "name": "",
      "born": 0
    }
  }
}`
	if got["ListBooks"] != "{\n  \"author\": \"\",\n  \"pageSize\": 0\n}" {
		t.Errorf("ListBooks template:\n%s", got["ListBooks"])
	}
	if got["ShelveBooks"] != "[\n"+indent(book)+"\n]" {
		t.Errorf("a client stream's template is an array of its message:\n%s", got["ShelveBooks"])
	}

	// Every template must be a message the method accepts as it stands.
	for i := range methods.Len() {
		md := methods.Get(i)
		if _, err := parseMessages(md, got[string(md.Name())], files); err != nil {
			t.Errorf("%s's template doesn't parse: %v", md.Name(), err)
		}
	}
}

func TestGRPCBlankMessageIsOneEmptyMessage(t *testing.T) {
	files, svc := librarySchema(t)
	for _, name := range []string{"GetBook", "ShelveBooks"} {
		in, err := parseMessages(svc.Methods().ByName(protoreflect.Name(name)), " \n", files)
		if err != nil || len(in) != 1 || proto.Size(in[0]) != 0 {
			t.Errorf("%s: blank parsed as %v, %v", name, in, err)
		}
	}
}

func TestGRPCCancelIsNotAResponse(t *testing.T) {
	addr := startLibrary(t, libraryOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	req := grpcRequest(addr, "library.v1.Library/ListBooks", `{"author": "stall"}`)
	_, err := NewGRPC("posting-test", TLSSettings{}, "").Send(ctx, Call{Request: req})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
}
