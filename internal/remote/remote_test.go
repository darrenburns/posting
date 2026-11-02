package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// socketDir is a short directory, since socket paths are limited to about
// 100 bytes and t.TempDir can be longer.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func TestCallsReachTheHandler(t *testing.T) {
	dir := socketDir(t)
	server, err := Listen(dir, Info{PID: os.Getpid(), Version: "3", Collection: "/c"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	server.Serve(HandlerFunc(func(_ context.Context, req Request) (any, error) {
		if req.Command == "fail" {
			return nil, errors.New("no good")
		}
		return Shown{YAML: req.Command + " " + req.Ref}, nil
	}))

	var shown Shown
	if err := Call(context.Background(), server.Path(), Request{Command: CommandShow, Ref: "a"}, &shown); err != nil || shown.YAML != "show a" {
		t.Errorf("show gave %+v, %v", shown, err)
	}
	if err := Call(context.Background(), server.Path(), Request{Command: "fail"}, nil); err == nil || err.Error() != "no good" {
		t.Errorf("a failing command gave %v", err)
	}

	instances, err := Instances(context.Background(), dir)
	if err != nil || len(instances) != 1 {
		t.Fatalf("instances = %+v, %v", instances, err)
	}
	if in := instances[0]; in.PID != os.Getpid() || in.Collection != "/c" || in.Socket != server.Path() {
		t.Errorf("instance = %+v", in)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("socket directory mode %v, %v", info.Mode().Perm(), err)
	}
}

func TestCommandsFromHalfClosedConnectionsAreAnswered(t *testing.T) {
	dir := socketDir(t)
	server, err := Listen(dir, Info{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	server.Serve(HandlerFunc(func(ctx context.Context, _ Request) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return Shown{YAML: "url: x"}, nil
		}
	}))
	// As `echo '{"command": "show"}' | nc -U SOCKET` does.
	conn, err := net.Dial("unix", server.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write([]byte(`{"command": "show"}` + "\n"))
	conn.(*net.UnixConn).CloseWrite()
	var reply Reply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil || !reply.OK || !strings.Contains(string(reply.Result), "url: x") {
		t.Errorf("reply = %+v, %v", reply, err)
	}
}

func TestCloseAbandonsCommands(t *testing.T) {
	dir := socketDir(t)
	server, err := Listen(dir, Info{})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	server.Serve(HandlerFunc(func(ctx context.Context, _ Request) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	failed := make(chan error, 1)
	go func() { failed <- Call(context.Background(), server.Path(), Request{Command: CommandSend}, nil) }()
	<-started
	server.Close()
	select {
	case err := <-failed:
		if err == nil {
			t.Error("a command abandoned by Close succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Error("the client wasn't let go when Posting closed")
	}
}

func TestCloseRemovesTheSocket(t *testing.T) {
	dir := socketDir(t)
	server, err := Listen(dir, Info{})
	if err != nil {
		t.Fatal(err)
	}
	server.Serve(HandlerFunc(func(context.Context, Request) (any, error) { return nil, nil }))
	server.Close()
	if _, err := os.Stat(server.Path()); !os.IsNotExist(err) {
		t.Errorf("socket still there after Close: %v", err)
	}
}

func TestInstancesRemovesStaleSockets(t *testing.T) {
	dir := socketDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "1.sock")
	listener, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	// Leave the file behind, as a process that was killed does.
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	listener.Close()

	instances, err := Instances(context.Background(), dir)
	if err != nil || len(instances) != 0 {
		t.Errorf("instances = %+v, %v", instances, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale socket kept: %v", err)
	}
}

func TestListenTightensAnOpenDirectory(t *testing.T) {
	dir := socketDir(t)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o777)
	server, err := Listen(dir, Info{})
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	if info, _ := os.Stat(dir); info.Mode().Perm() != 0o700 {
		t.Errorf("directory left with mode %v", info.Mode().Perm())
	}
}

func TestChoose(t *testing.T) {
	a := Info{PID: 1, Socket: "/s/1.sock", Collection: "/work/api/collection", Cwd: "/work/api"}
	b := Info{PID: 2, Socket: "/s/2.sock", Collection: "/home/me/default", Cwd: "/home/me"}
	c := Info{PID: 3, Socket: "/s/3.sock", Collection: "/work/web/requests", Cwd: "/elsewhere"}
	all := []Info{a, b, c}
	for _, tc := range []struct {
		name, want, cwd string
		instances       []Info
		pid             int
		err             string
	}{
		{name: "none", instances: nil, err: "no running Posting"},
		{name: "only one", instances: []Info{c}, cwd: "/anywhere", pid: 3},
		{name: "by pid", instances: all, want: "2", pid: 2},
		{name: "by socket", instances: all, want: "/s/3.sock", pid: 3},
		{name: "unknown pid", instances: all, want: "9", err: "no running Posting with"},
		{name: "started here", instances: all, cwd: "/home/me", pid: 2},
		{name: "inside a collection", instances: all, cwd: "/work/web/requests/users", pid: 3},
		{name: "above a collection", instances: all, cwd: "/work/web", pid: 3},
		{name: "ambiguous", instances: all, cwd: "/work", err: "3 Postings are running"},
		{name: "a sibling with a common prefix", instances: []Info{a, c}, cwd: "/work/webapp", err: "2 Postings"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Choose(tc.instances, tc.want, tc.cwd)
			switch {
			case tc.err != "":
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Errorf("err = %v, want %q", err, tc.err)
				}
			case err != nil:
				t.Error(err)
			case got.PID != tc.pid:
				t.Errorf("chose %d, want %d", got.PID, tc.pid)
			}
		})
	}
}
