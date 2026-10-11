//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/remote"
)

// TestSignalsRemoveTheSocket stops a stand-in for Posting with the signals
// that end it without its deferred cleanup, such as the hangup sent when its
// terminal closes. Each must remove its socket and still end it.
func TestSignalsRemoveTheSocket(t *testing.T) {
	if dir := os.Getenv("POSTING_TEST_SOCKET_DIR"); dir != "" {
		server, err := remote.Listen(dir, remote.Info{})
		if err != nil {
			os.Exit(3)
		}
		server.Serve(remote.HandlerFunc(func(context.Context, remote.Request) (any, error) { return nil, nil }))
		defer closeOnSignal(server)()
		os.WriteFile(filepath.Join(dir, "ready"), nil, 0o600)
		time.Sleep(time.Minute)
		os.Exit(4)
	}
	for _, sig := range []syscall.Signal{syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			dir, err := os.MkdirTemp("", "pr")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			cmd := exec.Command(os.Args[0], "-test.run=^TestSignalsRemoveTheSocket$")
			cmd.Env = append(os.Environ(), "POSTING_TEST_SOCKET_DIR="+dir)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			socket := filepath.Join(dir, "*.sock")
			if found, _ := filepath.Glob(socket); len(found) != 1 {
				t.Fatalf("no socket before the signal: %v", found)
			}
			cmd.Process.Signal(sig)
			err = cmd.Wait()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != sig {
				t.Errorf("the process ended with %v, want killed by %v", err, sig)
			}
			if found, _ := filepath.Glob(socket); len(found) != 0 {
				t.Errorf("socket left behind: %v", found)
			}
		})
	}
}
