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

	"github.com/darrenburns/posting/internal/remote"
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
}
