package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/remote"
)

func TestParseRemoteOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want remoteOptions
		err  string
	}{
		{args: []string{"send", "users/get", "--json"}, want: remoteOptions{command: "send", ref: "users/get", json: true}},
		{args: []string{"--instance", "42", "send", "--curl", "curl x"}, want: remoteOptions{command: "send", curl: "curl x", instance: "42"}},
		{args: []string{"open", "--yaml=-"}, want: remoteOptions{command: "open", yaml: "-"}},
		{args: []string{"send", "--", "-odd-name"}, want: remoteOptions{command: "send", ref: "-odd-name"}},
		{args: []string{"list"}, want: remoteOptions{command: "list"}},
		{args: []string{"env", "staging"}, want: remoteOptions{command: "env", env: []string{"staging"}}},
		{args: []string{"env", "--none"}, want: remoteOptions{command: "env", env: []string{}, noEnv: true}},
		{args: []string{"save", "users/new", "--name", "New user"}, want: remoteOptions{command: "save", file: "users/new", name: "New user"}},
		{args: []string{"env", "staging", "--none"}, err: "not both"},
		{args: []string{"send", "--none"}, err: "--none is for env"},
		{args: []string{"send", "--name", "x"}, err: "--name is for save"},
		{args: []string{"save", "a", "b"}, err: `unexpected argument "b"`},
		{args: []string{"save", "--curl", "curl x"}, err: "for open and send"},
		{args: nil, err: "no command"},
		{args: []string{"dance"}, err: "unknown command"},
		{args: []string{"send", "a", "b"}, err: `unexpected argument "b"`},
		{args: []string{"send", "a", "--curl", "curl x"}, err: "only one"},
		{args: []string{"show", "a"}, err: "unexpected argument"},
		{args: []string{"show", "--curl", "x"}, err: "for open and send"},
		{args: []string{"send", "--curl"}, err: "needs a value"},
		{args: []string{"send", "--curl="}, err: "needs a nonempty value"},
		{args: []string{"send", "--curl", " \t\n"}, err: "needs a nonempty value"},
		{args: []string{"send", "--yaml="}, err: "needs a nonempty value"},
		{args: []string{"send", "--verbose"}, err: "unknown option"},
	} {
		got, err := parseRemoteOptions(tc.args)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%q: err = %v, want %q", tc.args, err, tc.err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q = %+v, %v; want %+v", tc.args, got, err, tc.want)
		}
	}
}

func TestRemoteHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := remoteCommand([]string{"--help"}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "posting remote send") {
		t.Errorf("--help exited %d with %q", code, stdout.String())
	}
	if code := remoteCommand([]string{"dance"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("an unknown command exited %d", code)
	}
}

func TestRemoteRequestReadsFilesOutsideTheCollection(t *testing.T) {
	dir := t.TempDir()
	coll := filepath.Join(dir, "collection")
	os.MkdirAll(filepath.Join(coll, "users"), 0o755)
	inside := filepath.Join(coll, "users", "get.posting.yaml")
	outside := filepath.Join(dir, "draft.posting.yaml")
	os.WriteFile(inside, []byte("url: https://inside\n"), 0o644)
	os.WriteFile(outside, []byte("url: https://outside\n"), 0o644)
	instance := remote.Info{Collection: coll}

	if _, err := remoteRequest(remoteOptions{command: "save", file: outside}, instance, nil); err == nil {
		t.Error("saved outside the collection")
	}
	if _, err := remoteRequest(remoteOptions{command: "env", env: []string{"staging", "prod"}}, instance, nil); err == nil {
		t.Error("switched to two environments by name")
	}
	for _, tc := range []struct {
		opts remoteOptions
		want remote.Request
	}{
		{remoteOptions{command: "send", ref: inside}, remote.Request{Command: "send", Ref: "users/get.posting.yaml"}},
		{remoteOptions{command: "send", ref: outside}, remote.Request{Command: "send", YAML: "url: https://outside\n"}},
		{remoteOptions{command: "send", ref: "users/get"}, remote.Request{Command: "send", Ref: "users/get"}},
		{remoteOptions{command: "open", yaml: outside}, remote.Request{Command: "open", YAML: "url: https://outside\n"}},
		{remoteOptions{command: "open", yaml: "-"}, remote.Request{Command: "open", YAML: "url: https://stdin\n"}},
		{remoteOptions{command: "save", file: "users/new"}, remote.Request{Command: "save", File: "users/new"}},
		{remoteOptions{command: "save", file: filepath.Join(coll, "users", "new.posting.yaml")}, remote.Request{Command: "save", File: "users/new.posting.yaml"}},
		{remoteOptions{command: "env", env: []string{"staging"}}, remote.Request{Command: "env", Environment: "staging"}},
		{remoteOptions{command: "env", env: []string{outside, inside}}, remote.Request{Command: "env", EnvironmentFiles: []string{outside, inside}}},
		{remoteOptions{command: "env", noEnv: true}, remote.Request{Command: "env", NoEnvironment: true}},
	} {
		got, err := remoteRequest(tc.opts, instance, strings.NewReader("url: https://stdin\n"))
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%+v = %+v, %v; want %+v", tc.opts, got, err, tc.want)
		}
	}
}

func TestRemoteSaveResolvesSymlinkParents(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	coll := filepath.Join(dir, "collection")
	outside := filepath.Join(dir, "outside")
	for _, path := range []string{filepath.Join(coll, "users"), outside} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(dir, "alias")
	for _, link := range []struct{ target, path string }{
		{coll, alias},
		{filepath.Join(coll, "users"), filepath.Join(coll, "linked-users")},
		{outside, filepath.Join(coll, "escape")},
		{filepath.Join(outside, "missing"), filepath.Join(coll, "dangling")},
		{filepath.Join(coll, "loop"), filepath.Join(coll, "loop")},
	} {
		if err := os.Symlink(link.target, link.path); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, collection, file, want string
	}{
		{"aliased collection", alias, filepath.Join(alias, "users/new.posting.yaml"), "users/new.posting.yaml"},
		{"aliased file", coll, filepath.Join(alias, "users/new.posting.yaml"), "users/new.posting.yaml"},
		{"canonical file", alias, filepath.Join(coll, "users/new.posting.yaml"), "users/new.posting.yaml"},
		{"new directories", alias, filepath.Join(alias, "new/nested/request.posting.yaml"), "new/nested/request.posting.yaml"},
		{"linked directory", coll, filepath.Join(coll, "linked-users/new.posting.yaml"), "users/new.posting.yaml"},
		{"outside collection", coll, filepath.Join(outside, "new.posting.yaml"), ""},
		{"escaped directory", coll, filepath.Join(coll, "escape/new.posting.yaml"), ""},
		{"escaped new directories", coll, filepath.Join(coll, "escape/new/nested/request.posting.yaml"), ""},
		{"dangling symlink", coll, filepath.Join(coll, "dangling"), ""},
		{"dangling parent", coll, filepath.Join(coll, "dangling/new.posting.yaml"), ""},
		{"symlink loop", coll, filepath.Join(coll, "loop/new.posting.yaml"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := remoteRequest(remoteOptions{command: "save", file: tc.file}, remote.Info{Collection: tc.collection}, nil)
			if tc.want == "" {
				if err == nil || got.File != "" {
					t.Fatalf("accepted save outside the collection: %+v, %v", got, err)
				}
				return
			}
			if err != nil || got.File != tc.want {
				t.Fatalf("save file = %q, %v; want %q", got.File, err, tc.want)
			}
		})
	}
}

func TestRemoteRejectsEmptyRequestSources(t *testing.T) {
	dir, err := os.MkdirTemp("", "pr")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("POSTING_INSTANCE", "")
	server, err := remote.Listen(filepath.Join(dir, "posting"), remote.Info{PID: os.Getpid(), Collection: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	sent := make(chan remote.Request, 16)
	server.Serve(remote.HandlerFunc(func(_ context.Context, req remote.Request) (any, error) {
		sent <- req
		return remote.Exchange{Outcome: remote.OutcomeDone, Response: &remote.Response{Status: "200", Body: "hi"}}, nil
	}))
	empty := filepath.Join(t.TempDir(), "empty.posting.yaml")
	space := filepath.Join(t.TempDir(), "space.posting.yaml")
	for file, content := range map[string]string{empty: "", space: " \t\n"} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, stdin string
		args        []string
	}{
		{name: "empty YAML file", args: []string{"send", "--yaml", empty}},
		{name: "whitespace YAML file", args: []string{"send", "--yaml", space}},
		{name: "empty stdin", args: []string{"send", "--yaml", "-"}},
		{name: "whitespace stdin", stdin: " \t\n", args: []string{"send", "--yaml", "-"}},
		{name: "empty outside request", args: []string{"send", empty}},
		{name: "whitespace outside request", args: []string{"send", space}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := remoteCommand(tc.args, strings.NewReader(tc.stdin), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "request YAML is empty") {
				t.Fatalf("empty source exited %d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			select {
			case req := <-sent:
				t.Fatalf("empty source sent command %+v", req)
			default:
			}
		})
	}
	var stdout, stderr bytes.Buffer
	if code := remoteCommand([]string{"send"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("send active request exited %d: %s", code, stderr.String())
	}
	if req := <-sent; req.Command != remote.CommandSend || req.Ref != "" || req.Curl != "" || req.YAML != "" {
		t.Fatalf("send active request sent %+v", req)
	}
}

func TestPrintExchange(t *testing.T) {
	e := remote.Exchange{Outcome: remote.OutcomeDone, Response: &remote.Response{
		Proto: "HTTP/1.1", Status: "404", Reason: "Not Found", StatusCode: 404,
		Headers:     []remote.Header{{Name: "Content-Type", Value: "application/json"}},
		ContentType: "application/json", Body: `{"error":"missing"}`,
	}}
	var stdout, stderr bytes.Buffer
	if code := printExchange(e, false, &stdout, &stderr); code != 0 {
		t.Errorf("an error status exited %d", code)
	}
	want := "HTTP/1.1 404 Not Found\nContent-Type: application/json\n\n{\n  \"error\": \"missing\"\n}\n"
	if stdout.String() != want {
		t.Errorf("printed %q, want %q", stdout.String(), want)
	}

	stdout.Reset()
	failed := remote.Exchange{Outcome: remote.OutcomeFailed, Error: "connection refused"}
	if code := printExchange(failed, false, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "connection refused") {
		t.Errorf("a failure exited %d with %q", code, stderr.String())
	}
}

// TestRemoteCommandEndToEnd drives a stand-in for a running Posting
// through the command line.
func TestRemoteCommandEndToEnd(t *testing.T) {
	dir, err := os.MkdirTemp("", "pr")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("POSTING_INSTANCE", "")
	server, err := remote.Listen(filepath.Join(dir, "posting"), remote.Info{PID: os.Getpid(), Collection: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var got remote.Request
	server.Serve(remote.HandlerFunc(func(_ context.Context, req remote.Request) (any, error) {
		got = req
		return remote.Exchange{Outcome: remote.OutcomeDone, Response: &remote.Response{Status: "200", Body: "hi"}}, nil
	}))

	var stdout, stderr bytes.Buffer
	if code := remoteCommand([]string{"send", "users/get", "--json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("send exited %d: %s", code, stderr.String())
	}
	var e remote.Exchange
	if err := json.Unmarshal(stdout.Bytes(), &e); err != nil || e.Response.Body != "hi" {
		t.Errorf("printed %q (%v)", stdout.String(), err)
	}
	if got.Command != "send" || got.Ref != "users/get" {
		t.Errorf("Posting got %+v", got)
	}

	stdout.Reset()
	if code := remoteCommand([]string{"list"}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), dir) {
		t.Errorf("list exited %d with %q", code, stdout.String())
	}

	// From a sandbox that sees another socket directory, a socket given by
	// path still works.
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	stdout.Reset()
	if code := remoteCommand([]string{"send", "users/get"}, nil, &stdout, &stderr); code != 1 {
		t.Errorf("send without the socket directory exited %d", code)
	}
	t.Setenv("POSTING_INSTANCE", server.Path())
	stdout.Reset()
	if code := remoteCommand([]string{"send", "users/get"}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "hi") {
		t.Errorf("send by socket path exited %d with %q", code, stdout.String())
	}
}

func TestRemoteSkillPrintsTheEmbeddedSkill(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := remoteCommand([]string{"skill"}, nil, &stdout, &stderr); code != 0 || !strings.HasPrefix(stdout.String(), "---\nname: using-posting\n") {
		t.Errorf("skill exited %d with %.40q", code, stdout.String())
	}
}
