package main

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/paths"
)

func runTestDir(t *testing.T) string {
	t.Helper()
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(strings.ToUpper(name), "POSTING_") {
			t.Setenv(name, "")
		}
	}
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(name, t.TempDir())
	}
	return t.TempDir()
}

func writeRunFile(t *testing.T, root, file, data string) {
	t.Helper()
	path := filepath.Join(root, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runJSON(t *testing.T, exit int, args ...string) runReport {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args = append([]string{"run", "--json"}, args...)
	if code := run(args, &stdout, &stderr); code != exit {
		t.Fatalf("%v exited %d, want %d\nstdout: %s\nstderr: %s", args, code, exit, &stdout, &stderr)
	}
	var report runReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON report: %v\n%s", err, &stdout)
	}
	return report
}

func TestRunCollectionChainsInFileOrder(t *testing.T) {
	dir := runTestDir(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			if r.Method != "POST" {
				t.Errorf("login method = %s", r.Method)
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie", Path: "/"})
			fmt.Fprint(w, `{"token":"captured-token","id":9007199254740993}`)
		case "/users/9007199254740993":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "cookie" || r.Header.Get("Authorization") != "Bearer captured-token" {
				w.WriteHeader(http.StatusUnauthorized)
			}
			fmt.Fprint(w, `{"name":"Ada"}`)
		case "/upload":
			body, _ := io.ReadAll(r.Body)
			if r.Method != "PUT" || string(body) != "upload payload" {
				w.WriteHeader(http.StatusBadRequest)
			}
			fmt.Fprint(w, `{"uploaded":true}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	writeRunFile(t, dir, "posting.env", "BASE="+server.URL+"\nTOKEN=environment-token\n")
	writeRunFile(t, dir, "01-login.posting.yaml", `name: Z login
method: POST
url: ${BASE}/login
captures:
  TOKEN: token
  ID: id
`)
	writeRunFile(t, dir, "02-user.posting.yaml", `name: A user
url: ${BASE}/users/${ID}
auth:
  type: bearer_token
  bearer_token:
    token: ${TOKEN}
variable_scope:
  variables:
    TOKEN: request-token
`)
	writeRunFile(t, dir, "nested/03-upload.posting.yaml", "method: PUT\nurl: ${BASE}/upload\nbody:\n  file: payload.txt\n")
	writeRunFile(t, dir, "payload.txt", "upload payload")
	// Ignored directories must not become extra calls or parse failures.
	writeRunFile(t, dir, ".hidden/bad.posting.yaml", "invalid: [")
	writeRunFile(t, dir, "node_modules/bad.posting.yaml", "invalid: [")
	report := runJSON(t, 0, dir)
	if report.Passed != 3 || report.Failed != 0 || report.Skipped != 0 || calls.Load() != 3 {
		t.Fatalf("report = %+v; calls = %d", report, calls.Load())
	}
	var files []string
	for _, result := range report.Results {
		files = append(files, result.File)
		if result.Outcome != "passed" || result.Response == nil || result.Response.StatusCode != 200 {
			t.Fatalf("result = %+v", result)
		}
	}
	if want := []string{"01-login.posting.yaml", "02-user.posting.yaml", "nested/03-upload.posting.yaml"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("order = %v, want %v", files, want)
	}
	if body := report.Results[1].Response.Body; body != `{"name":"Ada"}` {
		t.Fatalf("response body = %q", body)
	}
	// A separate invocation cannot reuse captures, even in the same process.
	report = runJSON(t, 1, "-c", dir, "02-user.posting.yaml")
	if report.Failed != 1 || !strings.Contains(report.Results[0].Error, "$ID") || calls.Load() != 3 {
		t.Fatalf("captures leaked between runs: %+v", report)
	}
	for _, path := range []string{paths.HistoryDir(), paths.SocketDir(), paths.EnvironmentMemory()} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("headless run created %s: %v", path, err)
		}
	}
}

func TestRunEnvironmentsTLSAndSingleFile(t *testing.T) {
	dir := runTestDir(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("RUN_TEST_SECRET", "shell-secret")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%s|%s", r.Header.Get("X-Layer"), r.Header.Get("Authorization"))
	}))
	defer server.Close()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	writeRunFile(t, dir, "ca.pem", string(ca))
	t.Setenv("POSTING_SSL__CA_BUNDLE", filepath.Join(dir, "ca.pem"))
	writeRunFile(t, dir, "posting.env", "BASE="+server.URL+"\nLAYER=base\nTOKEN=${RUN_TEST_SECRET}\n")
	writeRunFile(t, dir, "posting.local.env", "LAYER=base-local\n")
	writeRunFile(t, dir, "ci.env", "LAYER=ci\n")
	writeRunFile(t, dir, "ci.local.env", "LAYER=ci-local\n")
	writeRunFile(t, cwd, "override.env", "LAYER=override\n")
	writeRunFile(t, cwd, "only.env", "BASE="+server.URL+"\nLAYER=only\nTOKEN=file-token\n")
	writeRunFile(t, cwd, "posting.env", "BASE=http://wrong.invalid\n")
	writeRunFile(t, cwd, "ci.env", "BASE=http://wrong.invalid\n")
	writeRunFile(t, dir, "health.posting.yaml", `url: ${BASE}/health
headers:
- name: X-Layer
  value: ${LAYER}
auth:
  type: bearer_token
  bearer_token:
    token: ${TOKEN}
`)
	if err := (env.Memory{File: paths.EnvironmentMemory()}).Remember(dir, []string{filepath.Join(cwd, "posting.env")}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		args       []string
	}{
		{"file default ignores cwd and remembered environment", "base-local|Bearer shell-secret", []string{filepath.Join(dir, "health.posting.yaml")}},
		{"named environment in collection", "ci-local|Bearer shell-secret", []string{"-c", dir, "health.posting.yaml", "-e", "ci"}},
		{"layered files", "override|Bearer shell-secret", []string{dir, "--env=ci", "-e", "override.env"}},
		{"explicit file has no implicit base", "only|Bearer file-token", []string{dir, "-e", "only.env"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := runJSON(t, 0, tc.args...)
			if report.Passed != 1 || len(report.Results) != 1 || report.Results[0].Response.Body != tc.body {
				t.Fatalf("report = %+v; want body %q", report, tc.body)
			}
		})
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "-c", dir}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "PASSED health.posting.yaml: 200 OK") || !strings.Contains(stdout.String(), "1 passed, 0 failed, 0 skipped") {
		t.Fatalf("text run exited %d: %s / %s", code, &stdout, &stderr)
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatal("text output printed the response body")
	}
}

func TestRunFailuresAndFailFast(t *testing.T) {
	dir := runTestDir(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/http-error":
			w.WriteHeader(400)
		case "/graphql-error":
			fmt.Fprint(w, `{"data":null,"errors":[{"message":"denied"}]}`)
		case "/graphql-ok":
			fmt.Fprint(w, `{"data":{"ok":true},"errors":[]}`)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusTemporaryRedirect)
		case "/binary":
			w.Write([]byte{0xff, 0x00, 0x10})
		case "/sse", "/sse-slow":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: retained\n\n")
			w.(http.Flusher).Flush()
			if r.URL.Path == "/sse-slow" {
				<-r.Context().Done()
			}
		case "/slow":
			<-r.Context().Done()
		default:
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	defer server.Close()
	writeRunFile(t, dir, "posting.env", "BASE="+server.URL+"\n")
	writeRunFile(t, dir, "02-control.posting.yaml", "url: ${BASE}/ok\n")
	for _, tc := range []struct {
		name, yaml, errorText string
		success               bool
	}{
		{"http error", "url: ${BASE}/http-error\n", "400", false},
		{"graphql error", "kind: graphql\nurl: ${BASE}/graphql-error\ngraphql:\n  query: '{ ok }'\n", "1 error", false},
		{"graphql success", "kind: graphql\nurl: ${BASE}/graphql-ok\ngraphql:\n  query: '{ ok }'\n", "", true},
		{"redirect without following", "url: ${BASE}/redirect\noptions:\n  follow_redirects: false\n", "", true},
		{"capture failure", "url: ${BASE}/ok\ncaptures:\n  MISSING: missing\n", "matched 0", false},
		{"undefined URL", "url: ${RUN_TEST_UNDEFINED_BASE}/ok\n", "Variable not defined", false},
		{"timeout", "url: ${BASE}/slow\noptions:\n  timeout: 0.02\n", "timed out", false},
		{"binary body", "url: ${BASE}/binary\n", "", true},
		{"finite SSE", "url: ${BASE}/sse\n", "", true},
		{"SSE timeout", "url: ${BASE}/sse-slow\noptions:\n  timeout: 0.1\n", "Stream interrupted", false},
	} {
		for _, failFast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fast=%t", tc.name, failFast), func(t *testing.T) {
				writeRunFile(t, dir, "01-case.posting.yaml", tc.yaml)
				args := []string{dir}
				if failFast {
					args = append(args, "--fail-fast")
				}
				exit, failed, passed, skipped := 1, 1, 1, 0
				if tc.success {
					exit, failed, passed = 0, 0, 2
				} else if failFast {
					passed, skipped = 0, 1
				}
				calls.Store(0)
				report := runJSON(t, exit, args...)
				if report.Failed != failed || report.Passed != passed || report.Skipped != skipped || len(report.Results) != 2-skipped {
					t.Fatalf("report = %+v", report)
				}
				result := report.Results[0]
				if !strings.Contains(result.Error, tc.errorText) {
					t.Fatalf("error = %q, want %q", result.Error, tc.errorText)
				}
				wantCalls := 2 - skipped
				if tc.name == "undefined URL" {
					wantCalls--
				}
				if calls.Load() != int32(wantCalls) {
					t.Fatalf("made %d calls, want %d", calls.Load(), wantCalls)
				}
				if tc.name == "binary body" && (result.Response.Body != "" || result.Response.BodyBase64 != "/wAQ") {
					t.Fatalf("binary response = %+v", result.Response)
				}
				if tc.name == "finite SSE" || tc.name == "SSE timeout" {
					if result.Response == nil || result.Response.Body != "data: retained\n\n" || tc.name == "SSE timeout" && !strings.Contains(result.Response.StreamError, "timed out") {
						t.Fatalf("SSE response lost data or timeout details: %+v", result.Response)
					}
				}
			})
		}
	}
}

func TestRunRejectsInvalidInputBeforeSending(t *testing.T) {
	dir := runTestDir(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	writeRunFile(t, dir, "01-valid.posting.yaml", "url: "+server.URL+"\n")
	for _, bad := range []string{"invalid: [", "", "{}", "url: " + server.URL + "\nscripts:\n  on_response: assertions.py\n"} {
		writeRunFile(t, dir, "02-bad.posting.yaml", bad)
		report := runJSON(t, 1, dir)
		if report.Error == "" || len(report.Results) != 0 || calls.Load() != 0 {
			t.Fatalf("invalid collection was sent: %+v, calls=%d", report, calls.Load())
		}
	}
	for _, args := range [][]string{{t.TempDir()}, {filepath.Join(dir, "missing.posting.yaml")}, {"-c", dir, "01-valid.posting.yaml", "--env", "missing"}} {
		report := runJSON(t, 1, args...)
		if report.Error == "" || calls.Load() != 0 {
			t.Fatalf("invalid input accepted: %+v", report)
		}
	}
	for _, args := range [][]string{{}, {""}, {"a", "b"}, {"--unknown"}, {"--env"}, {"--collection="}, {"--json=false"}} {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"run"}, args...), &stdout, &stderr); code != 2 {
			t.Fatalf("%v exited %d, want 2: %s", args, code, &stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "Usage: posting run") {
		t.Fatalf("help exited %d: %s / %s", code, &stdout, &stderr)
	}
}

func TestRunGRPCWithCollectionRelativeSchema(t *testing.T) {
	dir := runTestDir(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, health.NewServer())
	go server.Serve(listener)
	defer server.Stop()
	// No reflection: resolving this schema against the collection root is
	// required even though the selected request is in a nested directory.
	writeRunFile(t, dir, "protos/health.proto", `syntax = "proto3";
package grpc.health.v1;
message HealthCheckRequest { string service = 1; }
message HealthCheckResponse { int32 status = 1; }
service Health { rpc Check(HealthCheckRequest) returns (HealthCheckResponse); }
`)
	for _, tc := range []struct {
		service, status string
		exit            int
	}{
		{"", "OK", 0},
		{"unknown-service", "NOT_FOUND", 1},
	} {
		writeRunFile(t, dir, "nested/health.posting.yaml", fmt.Sprintf(`kind: grpc
url: %s
grpc:
  method: grpc.health.v1.Health/Check
  message: '{"service":%q}'
  proto:
    files: [protos/health.proto]
`, listener.Addr(), tc.service))
		report := runJSON(t, tc.exit, "-c", dir, "nested")
		if len(report.Results) != 1 || report.Results[0].Response == nil || report.Results[0].Response.Status != tc.status {
			t.Fatalf("gRPC result = %+v", report)
		}
		if tc.exit == 0 {
			var body struct{ Status int }
			if err := json.Unmarshal([]byte(report.Results[0].Response.Body), &body); err != nil || body.Status != 1 {
				t.Fatalf("gRPC body = %s (%v)", report.Results[0].Response.Body, err)
			}
		}
	}
}
