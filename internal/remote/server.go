package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Handler carries out commands for a Server. Handle may block until the
// command is done, and should give up when ctx ends, which it does when the
// Server closes. The result is sent back as JSON.
type Handler interface {
	Handle(ctx context.Context, req Request) (any, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, req Request) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, req Request) (any, error) { return f(ctx, req) }

// Server listens for commands on a Unix socket.
type Server struct {
	listener net.Listener
	path     string
	info     Info
	ctx      context.Context
	stop     context.CancelFunc
	wg       sync.WaitGroup
}

// Listen starts listening in dir, on a socket named for this process. Info
// answers CommandInfo. dir is created private to the user, and refused if
// it's anyone else's.
func Listen(dir string, info Info) (*Server, error) {
	if err := privateDir(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, strconv.Itoa(os.Getpid())+".sock")
	// A socket left by a process that had this ID before, and is gone.
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	ctx, stop := context.WithCancel(context.Background())
	return &Server{listener: listener, path: path, info: info, ctx: ctx, stop: stop}, nil
}

// Path is the socket's path.
func (s *Server) Path() string { return s.path }

// Serve answers connections with h until Close. It returns at once.
func (s *Server) Serve(h Handler) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.serveConn(conn, h)
			}()
		}
	}()
}

// Close stops listening, abandons commands still being handled and removes
// the socket.
func (s *Server) Close() error {
	s.stop()
	err := s.listener.Close()
	// Unix listeners remove their socket on Close, but not always on every
	// platform.
	_ = os.Remove(s.path)
	s.wg.Wait()
	return err
}

func (s *Server) serveConn(conn net.Conn, h Handler) {
	defer conn.Close()
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeReply(conn, nil, fmt.Errorf("couldn't read command: %w", err))
		return
	}
	// A client that goes away isn't noticed: what it asked for is done in
	// the UI anyway, and clients that close their sending side after the
	// command, as nc does, still get the reply.
	var (
		result any
		err    error
	)
	if req.Command == CommandInfo {
		result = s.info
	} else {
		result, err = h.Handle(ctx, req)
	}
	if ctx.Err() != nil {
		return
	}
	writeReply(conn, result, err)
}

func writeReply(w io.Writer, result any, err error) {
	reply := Reply{OK: err == nil}
	if err != nil {
		reply.Error = err.Error()
	} else if result != nil {
		data, merr := json.Marshal(result)
		if merr != nil {
			reply = Reply{Error: "couldn't encode result: " + merr.Error()}
		} else {
			reply.Result = data
		}
	}
	_ = json.NewEncoder(w).Encode(reply)
}

// privateDir makes dir if it's missing, and checks no one else can use it:
// it must be a real directory, owned by this user, with no access for
// anyone else.
func privateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s isn't a directory", dir)
	}
	if !ownedByMe(info) {
		return fmt.Errorf("%s belongs to another user", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return errors.New(dir + " can be used by other users: " + err.Error())
		}
	}
	return nil
}
