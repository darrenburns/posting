package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Call sends req to the instance listening on socket and decodes its result
// into result, which may be nil. It waits as long as the command takes, or
// until ctx ends.
func Call(ctx context.Context, socket string, req Request, result any) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var reply Reply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("no reply from Posting: %w", err)
	}
	if !reply.OK {
		return errors.New(reply.Error)
	}
	if result == nil || len(reply.Result) == 0 {
		return nil
	}
	return json.Unmarshal(reply.Result, result)
}

// Instances lists the instances listening in dir, oldest first. Sockets
// left behind by instances that have exited are removed.
func Instances(ctx context.Context, dir string) ([]Info, error) {
	sockets, err := filepath.Glob(filepath.Join(dir, "*.sock"))
	if err != nil {
		return nil, err
	}
	var found []Info
	for _, socket := range sockets {
		var info Info
		probe, cancel := context.WithTimeout(ctx, time.Second)
		err := Call(probe, socket, Request{Command: CommandInfo}, &info)
		cancel()
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				_ = os.Remove(socket)
			}
			continue
		}
		info.Socket = socket
		found = append(found, info)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Started.Before(found[j].Started) })
	return found, nil
}

// Choose picks the instance to talk to from those running. want, when not
// empty, is a process ID or socket path naming it. Otherwise the only
// instance is chosen, or the only one started in cwd, or else the only one
// whose collection is in cwd or contains it.
func Choose(instances []Info, want, cwd string) (Info, error) {
	if len(instances) == 0 {
		return Info{}, errors.New("no running Posting found: start Posting in another terminal first")
	}
	if want != "" {
		for _, in := range instances {
			if fmt.Sprint(in.PID) == want || in.Socket == want {
				return in, nil
			}
		}
		return Info{}, fmt.Errorf("no running Posting with process ID or socket %s", want)
	}
	if len(instances) == 1 {
		return instances[0], nil
	}
	tests := []func(Info) bool{
		func(in Info) bool { return in.Cwd == cwd },
		func(in Info) bool { return within(cwd, in.Collection) || within(in.Collection, cwd) },
	}
	for _, test := range tests {
		var matches []Info
		for _, in := range instances {
			if test(in) {
				matches = append(matches, in)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	var list strings.Builder
	for _, in := range instances {
		fmt.Fprintf(&list, "\n  %d  %s (started in %s)", in.PID, in.Collection, in.Cwd)
	}
	return Info{}, fmt.Errorf("%d Postings are running; choose one with --instance PID:%s", len(instances), list.String())
}

// within reports whether path is dir or inside it.
func within(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
