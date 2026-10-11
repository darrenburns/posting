package ui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
	"github.com/darrenburns/posting/internal/remote"
)

// Remote carries out commands from `posting remote` (see package remote).
// Each runs on the UI goroutine, as if the user had done it, so the user
// sees requests open and send. Commands wait until the app is running.
func (a *App) Remote() remote.Handler { return remote.HandlerFunc(a.handleRemote) }

// markRunning lets remote commands through, once the app is running and
// dispatch reaches its goroutine.
func (a *App) markRunning() { a.runningOnce.Do(func() { close(a.running) }) }

func (a *App) handleRemote(ctx context.Context, req remote.Request) (any, error) {
	select {
	case <-a.running:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	switch req.Command {
	case remote.CommandRequests:
		return a.onUI(ctx, func() (any, error) { return a.savedRequests(), nil })
	case remote.CommandShow:
		return a.onUI(ctx, func() (any, error) { return a.shown(a.current()) })
	case remote.CommandOpen:
		return a.onUI(ctx, func() (any, error) {
			s, err := a.openRemote(req)
			if err != nil {
				return nil, err
			}
			return a.shown(s)
		})
	case remote.CommandSend:
		return a.sendRemote(ctx, req)
	case remote.CommandEnv:
		return a.onUI(ctx, func() (any, error) {
			if err := a.switchRemoteEnvironment(req); err != nil {
				return nil, err
			}
			return a.remoteEnvironments(), nil
		})
	case remote.CommandSave:
		return a.onUI(ctx, func() (any, error) {
			s, err := a.saveRemote(req)
			if err != nil {
				return nil, err
			}
			return a.shown(s)
		})
	case remote.CommandResponse:
		return a.onUI(ctx, func() (any, error) {
			s := a.current()
			if s == nil {
				return nil, errors.New("no request is open")
			}
			return a.exchange(s), nil
		})
	}
	return nil, fmt.Errorf("unknown command %q", req.Command)
}

// onUI runs fn on the UI goroutine and waits for its result.
func (a *App) onUI(ctx context.Context, fn func() (any, error)) (any, error) {
	type result struct {
		value any
		err   error
	}
	done := make(chan result, 1)
	a.dispatch(func() {
		value, err := fn()
		done <- result{value, err}
	})
	select {
	case r := <-done:
		return r.value, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendRemote opens and sends the request req names, and waits for the
// exchange to end.
func (a *App) sendRemote(ctx context.Context, req remote.Request) (any, error) {
	ended := make(chan remote.Exchange, 1)
	if _, err := a.onUI(ctx, func() (any, error) {
		s, err := a.openRemote(req)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(s.url.GetText()) == "" {
			return nil, errors.New("the request has no URL")
		}
		a.start(s, nil)
		if s.phase.Peek() != exchangeSending {
			// It failed before it was sent, such as for a missing variable.
			ended <- a.exchange(s)
			return nil, nil
		}
		s.whenSettled(func() { ended <- a.exchange(s) })
		return nil, nil
	}); err != nil {
		return nil, err
	}
	select {
	case e := <-ended:
		return e, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// openRemote shows the request a remote command names, and returns its tab.
// Saved requests open as if chosen in the sidebar. A request that isn't
// saved opens in the tab used for those, unless the user has edited it, so
// that a string of them doesn't leave a tab each. With none named, the
// active tab is used.
func (a *App) openRemote(req remote.Request) (*Session, error) {
	given := 0
	for _, v := range []string{req.Ref, req.Curl, req.YAML} {
		if v != "" {
			given++
		}
	}
	if given > 1 {
		return nil, errors.New("give only one of a saved request, a curl command or YAML")
	}
	if given == 0 {
		s := a.current()
		if s == nil {
			return nil, errors.New("no request is open")
		}
		return s, nil
	}
	var s *Session
	switch {
	case req.Ref != "":
		saved, err := a.findSaved(req.Ref)
		if err != nil {
			return nil, err
		}
		a.openRequest(saved)
		s = a.current()
	case req.Curl != "":
		r, err := curl.Parse(req.Curl)
		if err != nil {
			return nil, fmt.Errorf("couldn't import curl command: %w", err)
		}
		s = a.openUnsaved(r)
	default:
		r, err := collection.ParseRequest([]byte(req.YAML), "")
		if err != nil {
			return nil, fmt.Errorf("couldn't read request YAML: %w", err)
		}
		s = a.openUnsaved(r)
	}
	// Say why the tab changed under the user.
	a.notify("Opened "+s.title.Peek()+" from the command line", toastInfo)
	return s, nil
}

// openUnsaved opens req, which isn't saved, in the remote tab, or in the
// active tab if nothing has been done in it.
func (a *App) openUnsaved(req model.Request) *Session {
	req.File = ""
	if s := a.current(); s != nil && s.isPristine() && !s.preview.Peek() {
		s.Load(req)
		a.listProtoMethods(s)
		a.remoteTab = s.id
		return s
	}
	sessions := a.sessions.Peek()
	i := slices.IndexFunc(sessions, func(s *Session) bool { return s.id == a.remoteTab })
	if i < 0 || sessions[i].dirty.Peek() || sessions[i].file.Peek() != "" || sessions[i].phase.Peek() == exchangeSending {
		s := a.openSession(req)
		a.remoteTab = s.id
		a.sessionView.revealActive()
		return s
	}
	// A fresh session rather than reloading the old one, so its response
	// doesn't show beside a request it doesn't answer.
	s := a.createSession(req)
	sessions = slices.Clone(sessions)
	sessions[i] = s
	a.sessions.Set(sessions)
	a.remoteTab = s.id
	a.showSession(s.id)
	return s
}

// findSaved finds the saved request ref names: by its file, relative to the
// collection, with or without the .posting.yaml extension, or by its name.
func (a *App) findSaved(ref string) (model.Request, error) {
	ref = path.Clean(strings.ReplaceAll(ref, `\`, "/"))
	var byFile, byName []model.Request
	a.collection.Peek().Walk(func(_ *model.Collection, r model.Request) {
		switch {
		case r.File == ref || trimRequestExt(r.File) == trimRequestExt(ref):
			byFile = append(byFile, r)
		case strings.EqualFold(r.Name, ref):
			byName = append(byName, r)
		}
	})
	for _, matches := range [][]model.Request{byFile, byName} {
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], nil
		}
		files := make([]string, len(matches))
		for i, r := range matches {
			files[i] = r.File
		}
		return model.Request{}, fmt.Errorf("%q could be any of %s", ref, strings.Join(files, ", "))
	}
	return model.Request{}, fmt.Errorf("no request %q in the collection", ref)
}

func trimRequestExt(file string) string {
	for _, ext := range []string{".posting.yaml", ".posting.yml"} {
		if strings.HasSuffix(file, ext) {
			return strings.TrimSuffix(file, ext)
		}
	}
	return file
}

// switchRemoteEnvironment switches to the environment req names, if any.
// Unlike switching in the palette, it isn't remembered for the next launch:
// which environment Posting starts in stays the user's choice.
func (a *App) switchRemoteEnvironment(req remote.Request) error {
	given := 0
	for _, ok := range []bool{req.Environment != "", len(req.EnvironmentFiles) > 0, req.NoEnvironment} {
		if ok {
			given++
		}
	}
	switch {
	case given == 0:
		return nil
	case given > 1:
		return errors.New("give only one of an environment's name, its files or none")
	case req.NoEnvironment:
		a.setEnvironment(model.Environment{})
		a.notify("Environment cleared from the command line", toastInfo)
		return nil
	}
	files := req.EnvironmentFiles
	if req.Environment != "" {
		var matches [][]string
		for _, group := range a.environmentGroups() {
			if e, err := a.env.source.Load(group); err == nil && strings.EqualFold(e.Name, req.Environment) {
				matches = append(matches, group)
			}
		}
		switch len(matches) {
		case 0:
			return fmt.Errorf("no environment %q", req.Environment)
		case 1:
			files = matches[0]
		default:
			var choices []string
			for _, m := range matches {
				choices = append(choices, strings.Join(m, " + "))
			}
			return fmt.Errorf("%q could be any of %s; give its files instead", req.Environment, strings.Join(choices, ", "))
		}
	}
	loaded, err := a.env.source.Load(files)
	if err != nil {
		return fmt.Errorf("couldn't load environment: %w", err)
	}
	a.setEnvironment(loaded)
	a.notify("Switched to "+loaded.Name+" from the command line", toastInfo)
	return nil
}

func (a *App) remoteEnvironments() []remote.Environment {
	active := envKey(a.env.active.Peek().Files)
	out := []remote.Environment{}
	for _, files := range a.environmentGroups() {
		e := remote.Environment{Files: files, Active: envKey(files) == active, Variables: []string{}}
		if loaded, err := a.env.source.Load(files); err != nil {
			e.Name, e.Error = fileNames(files), err.Error()
		} else {
			e.Name = loaded.Name
			for _, v := range loaded.Variables {
				e.Variables = append(e.Variables, v.Name)
			}
		}
		out = append(out, e)
	}
	return out
}

// saveRemote saves the active tab's request: as req.File when given, and
// otherwise where it was saved before, or under its name in the
// collection's top folder. It won't replace another saved request.
func (a *App) saveRemote(req remote.Request) (*Session, error) {
	s := a.current()
	if s == nil {
		return nil, errors.New("no request is open")
	}
	snapshot := s.Snapshot()
	if req.Name != "" {
		snapshot.Name = strings.TrimSpace(req.Name)
	}
	file := s.file.Peek()
	switch {
	case req.File != "":
		file = strings.ReplaceAll(strings.TrimSpace(req.File), `\`, "/")
		if path.IsAbs(file) || strings.Contains(file, ":") || slices.Contains(strings.Split(file, "/"), "..") {
			return nil, fmt.Errorf("%s isn't a path inside the collection", req.File)
		}
		file = path.Clean(file)
		if !strings.HasSuffix(file, collection.FileSuffix) {
			file += collection.FileSuffix
		}
	case file == "":
		if slugify(snapshot.Name) == "" {
			return nil, errors.New("the request has no name to save it under; give a file")
		}
		file = slugify(snapshot.Name) + collection.FileSuffix
	}
	if file != s.file.Peek() && a.fileExists(file) {
		return nil, fmt.Errorf("a request is already saved as %s", file)
	}
	if snapshot.Name == "" {
		_, base := splitFile(file)
		snapshot.Name = strings.TrimSuffix(base, collection.FileSuffix)
	}
	snapshot.File = file
	if err := a.writeRequest(snapshot); err != nil {
		return nil, fmt.Errorf("couldn't save %s: %w", file, err)
	}
	// A saved tab is never reused for unsaved requests (see openUnsaved).
	s.savedAs(snapshot)
	a.notify("Saved "+file+" from the command line", toastSuccess)
	return s, nil
}

func (a *App) savedRequests() []remote.SavedRequest {
	out := []remote.SavedRequest{}
	a.collection.Peek().Walk(func(_ *model.Collection, r model.Request) {
		saved := remote.SavedRequest{File: r.File, Name: r.Name, Kind: string(r.Kind().ID), URL: r.URL}
		if r.Payload == nil {
			saved.Method = string(r.Method)
		}
		out = append(out, saved)
	})
	slices.SortFunc(out, func(x, y remote.SavedRequest) int { return strings.Compare(x.File, y.File) })
	return out
}

func (a *App) shown(s *Session) (remote.Shown, error) {
	if s == nil {
		return remote.Shown{}, errors.New("no request is open")
	}
	data, err := collection.MarshalRequest(s.Snapshot())
	if err != nil {
		return remote.Shown{}, err
	}
	return remote.Shown{Tab: tabOf(s), Environment: a.env.active.Peek().Name, YAML: string(data)}, nil
}

func tabOf(s *Session) remote.Tab {
	return remote.Tab{ID: s.id, Title: s.title.Peek(), File: s.file.Peek(), Dirty: s.dirty.Peek()}
}

// exchange is how s's latest exchange went.
func (a *App) exchange(s *Session) remote.Exchange {
	e := remote.Exchange{Tab: tabOf(s), Environment: a.env.active.Peek().Name}
	resp := s.response.Peek()
	switch s.phase.Peek() {
	case exchangeSending:
		e.Outcome = remote.OutcomeSending
		return e
	case exchangeFailed:
		e.Outcome = remote.OutcomeFailed
		if err := s.err.Peek(); err != nil {
			e.Error = err.Error()
		}
		return e
	case exchangeCancelled:
		e.Outcome = remote.OutcomeCancelled
		return e
	}
	// Done, or idle with a response opened from history.
	if resp == nil {
		e.Outcome = remote.OutcomeNone
		return e
	}
	e.Outcome = remote.OutcomeDone
	e.Response = remoteResponse(resp, s.responseStatus)
	return e
}

func remoteResponse(resp *model.Response, status model.Status) *remote.Response {
	out := &remote.Response{
		Method:      string(resp.Method),
		URL:         resp.URL,
		Status:      status.Code,
		Reason:      status.Text,
		StatusCode:  resp.StatusCode,
		Proto:       resp.Proto,
		Headers:     remoteHeaders(resp.Headers),
		Trailers:    remoteHeaders(resp.Trailers),
		ContentType: resp.ContentType(),
		Size:        resp.Size(),
		ElapsedMS:   float64(resp.Elapsed.Microseconds()) / 1000,
		ReceivedAt:  resp.ReceivedAt,
	}
	if utf8.Valid(resp.Body) {
		out.Body = string(resp.Body)
	} else {
		out.BodyBase64 = base64.StdEncoding.EncodeToString(resp.Body)
	}
	return out
}

func remoteHeaders(in []model.Header) []remote.Header {
	out := make([]remote.Header, len(in))
	for i, h := range in {
		out[i] = remote.Header{Name: h.Name, Value: h.Value}
	}
	return out
}
