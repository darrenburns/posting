package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
	"github.com/darrenburns/posting/internal/remote"
)

// remoteHarness carries out remote commands as the running app does: each
// command on a goroutine of its own, and the work it dispatches on the UI
// goroutine, which the test plays.
type remoteHarness struct {
	tt    *testing.T
	app   *App
	queue chan func()
}

func newRemoteHarness(tt *testing.T) *remoteHarness {
	tt.Helper()
	h := &remoteHarness{tt: tt, app: testApp(), queue: make(chan func(), 64)}
	h.app.dispatch = func(fn func()) { h.queue <- fn }
	for _, s := range h.app.sessions.Peek() {
		s.dispatch = h.app.dispatch
	}
	h.app.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) {
		return fixedResponse(), nil
	})
	h.app.markRunning()
	return h
}

func (h *remoteHarness) call(req remote.Request) (any, error) {
	h.tt.Helper()
	type result struct {
		value any
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := h.app.handleRemote(context.Background(), req)
		done <- result{value, err}
	}()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case fn := <-h.queue:
			fn()
		case r := <-done:
			return r.value, r.err
		case <-timeout:
			h.tt.Fatalf("remote %s didn't finish", req.Command)
		}
	}
}

func (h *remoteHarness) send(req remote.Request) remote.Exchange {
	h.tt.Helper()
	req.Command = remote.CommandSend
	result, err := h.call(req)
	if err != nil {
		h.tt.Fatalf("send: %v", err)
	}
	return result.(remote.Exchange)
}

func TestRemoteSendsSavedRequestWithTheActiveEnvironment(tt *testing.T) {
	h := newRemoteHarness(tt)
	var sent client.Call
	h.app.sender = client.SenderFunc(func(_ context.Context, call client.Call) (*model.Response, error) {
		sent = call
		return fixedResponse(), nil
	})

	e := h.send(remote.Request{Ref: "users/list-users"})

	if e.Outcome != remote.OutcomeDone || e.Response == nil {
		tt.Fatalf("outcome %q, response %v", e.Outcome, e.Response)
	}
	if e.Response.Status != "200" || e.Response.Reason != "OK" || !strings.Contains(e.Response.Body, `"Ada"`) {
		tt.Errorf("response = %+v", e.Response)
	}
	if e.Tab.File != "users/list-users.posting.yaml" || e.Tab.Title != "List users" || e.Environment != "local" {
		tt.Errorf("tab %+v in environment %q", e.Tab, e.Environment)
	}
	if sent.Request.Name != "List users" || sent.Variables["BASE_URL"] != "http://localhost:8000" {
		tt.Errorf("sent %q with BASE_URL %q", sent.Request.Name, sent.Variables["BASE_URL"])
	}
	if s := h.app.current(); s.file.Peek() != "users/list-users.posting.yaml" || s.response.Peek() == nil {
		tt.Error("the request and its response aren't showing in the active tab")
	}
	if n := len(h.app.history.Peek()); n != 1 {
		tt.Errorf("history has %d entries, want 1", n)
	}
}

func TestRemoteFindsSavedRequests(tt *testing.T) {
	h := newRemoteHarness(tt)
	for _, ref := range []string{"users/get-user.posting.yaml", "users/get-user", "./users/get-user", "get user"} {
		r, err := h.app.findSaved(ref)
		if err != nil || r.File != "users/get-user.posting.yaml" {
			tt.Errorf("findSaved(%q) = %q, %v", ref, r.File, err)
		}
	}
	if _, err := h.app.findSaved("users/nope"); err == nil {
		tt.Error("found a request that doesn't exist")
	}

	root := model.SampleCollection()
	twin := sampleRequest(tt, "Get user")
	twin.File = "auth/get-user.posting.yaml"
	root.Children[1].Requests = append(root.Children[1].Requests, twin)
	h.app.collection.Set(root)
	_, err := h.app.findSaved("Get user")
	if err == nil || !strings.Contains(err.Error(), "auth/get-user.posting.yaml") {
		tt.Errorf("an ambiguous name gave %v", err)
	}
}

func TestRemoteUnsavedRequestsShareATabUntilItIsEdited(tt *testing.T) {
	h := newRemoteHarness(tt)
	h.send(remote.Request{Ref: "Get user"})
	tabs := len(h.app.sessions.Peek())

	first := h.send(remote.Request{Curl: "curl -X POST http://localhost:8000/things -d '{}'"})
	if got := len(h.app.sessions.Peek()); got != tabs+1 {
		tt.Fatalf("%d tabs after the first unsaved request, want %d", got, tabs+1)
	}
	second := h.send(remote.Request{YAML: "method: DELETE\nurl: http://localhost:8000/things/1\n"})
	if got := len(h.app.sessions.Peek()); got != tabs+1 {
		tt.Errorf("%d tabs after the second unsaved request, want it to reuse the first's", got)
	}
	s := h.app.current()
	if s.id != second.Tab.ID || s.method.Peek() != model.MethodDelete || s.file.Peek() != "" {
		tt.Errorf("active tab %d is %s %s, want tab %d", s.id, s.method.Peek(), s.file.Peek(), second.Tab.ID)
	}
	if second.Tab.ID == first.Tab.ID {
		tt.Error("the reused tab kept its session, and so the old response")
	}

	s.markEdited()
	h.send(remote.Request{Curl: "curl http://localhost:8000/other"})
	if got := len(h.app.sessions.Peek()); got != tabs+2 {
		tt.Errorf("%d tabs, want a new one rather than replacing the edited tab", got)
	}
	if strings.TrimSpace(s.url.GetText()) != "http://localhost:8000/things/1" {
		tt.Errorf("the edited tab's URL became %q", s.url.GetText())
	}
}

func TestRemoteUnsavedRequestUsesAnUntouchedTab(tt *testing.T) {
	h := newRemoteHarness(tt)
	blank := h.app.current()
	e := h.send(remote.Request{Curl: "curl http://localhost:8000/things"})
	if len(h.app.sessions.Peek()) != 1 || e.Tab.ID != blank.id {
		tt.Errorf("sent from tab %d of %d, want the blank tab %d", e.Tab.ID, len(h.app.sessions.Peek()), blank.id)
	}
}

func TestRemoteReportsFailures(tt *testing.T) {
	h := newRemoteHarness(tt)
	h.app.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) {
		return nil, errors.New("connection refused")
	})
	if e := h.send(remote.Request{Ref: "users/list-users"}); e.Outcome != remote.OutcomeFailed || e.Error != "connection refused" || e.Response != nil {
		tt.Errorf("a failed send gave %+v", e)
	}
	h.app.sender = client.SenderFunc(func(_ context.Context, call client.Call) (*model.Response, error) {
		_, err := model.Resolve(call.Request, model.MapLookup(call.Variables))
		return fixedResponse(), err
	})
	if e := h.send(remote.Request{YAML: "url: ${MISSING}/x\n"}); e.Outcome != remote.OutcomeFailed || !strings.Contains(e.Error, "MISSING") {
		tt.Errorf("a missing variable gave %+v", e)
	}
	if _, err := h.call(remote.Request{Command: remote.CommandSend, YAML: "name: no url\n"}); err == nil {
		tt.Error("sent a request without a URL")
	}
	if _, err := h.call(remote.Request{Command: remote.CommandSend, Curl: "curl x", YAML: "url: x"}); err == nil {
		tt.Error("accepted two requests at once")
	}
	if _, err := h.call(remote.Request{Command: "dance"}); err == nil {
		tt.Error("accepted an unknown command")
	}
}

func TestRemoteShowsTheActiveTab(tt *testing.T) {
	h := newRemoteHarness(tt)
	result, err := h.call(remote.Request{Command: remote.CommandOpen, Ref: "Create user"})
	if err != nil {
		tt.Fatal(err)
	}
	opened := result.(remote.Shown)
	if opened.Tab.File != "users/create-user.posting.yaml" || !strings.Contains(opened.YAML, "url: ${BASE_URL}/users") {
		tt.Errorf("opened %+v", opened)
	}
	if h.app.current().response.Peek() != nil {
		tt.Error("open sent the request")
	}
	result, _ = h.call(remote.Request{Command: remote.CommandResponse})
	if e := result.(remote.Exchange); e.Outcome != remote.OutcomeNone {
		tt.Errorf("response before sending gave %q", e.Outcome)
	}

	h.send(remote.Request{})
	result, _ = h.call(remote.Request{Command: remote.CommandShow})
	if shown := result.(remote.Shown); shown.Tab.ID != opened.Tab.ID || shown.YAML != opened.YAML {
		tt.Errorf("show gave %+v, want the tab opened", shown)
	}
	result, _ = h.call(remote.Request{Command: remote.CommandResponse})
	if e := result.(remote.Exchange); e.Outcome != remote.OutcomeDone || e.Response.StatusCode != 200 {
		tt.Errorf("response after sending gave %+v", e)
	}

	result, _ = h.call(remote.Request{Command: remote.CommandRequests})
	if saved := result.([]remote.SavedRequest); len(saved) != 9 || saved[0].File != "auth/login.posting.yaml" || saved[0].Method != "POST" {
		tt.Errorf("requests gave %+v", saved)
	}
}

func TestRemoteSendEndsWhenItsTabCloses(tt *testing.T) {
	h := newRemoteHarness(tt)
	h.app.sender = client.SenderFunc(func(ctx context.Context, _ client.Call) (*model.Response, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	s := h.app.current()
	s.Load(sampleRequest(tt, "Health check"))
	var ended []remote.Exchange
	h.app.start(s, nil)
	s.whenSettled(func() { ended = append(ended, h.app.exchange(s)) })
	h.app.closeSession(s.id)
	if len(ended) != 1 || ended[0].Outcome != remote.OutcomeCancelled {
		tt.Errorf("closing the tab ended the send with %+v", ended)
	}
}

func TestRemoteWaitsForTheAppToRun(tt *testing.T) {
	app := testApp()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := app.handleRemote(ctx, remote.Request{Command: remote.CommandShow}); !errors.Is(err, context.DeadlineExceeded) {
		tt.Errorf("a command before the app ran gave %v", err)
	}
}

func TestRemoteResponseEncodesBinaryBodies(tt *testing.T) {
	resp := fixedResponse()
	resp.Body = []byte{0xff, 0x00, 0x10}
	out := remoteResponse(resp, model.StatusOf(model.NewRequest(), resp))
	if out.Body != "" || out.BodyBase64 != "/wAQ" || out.Size != 3 || out.ElapsedMS != 42 {
		tt.Errorf("binary body gave %+v", out)
	}
}

func TestRemoteSwitchesEnvironment(tt *testing.T) {
	h := newRemoteHarness(tt)
	var sent client.Call
	h.app.sender = client.SenderFunc(func(_ context.Context, call client.Call) (*model.Response, error) {
		sent = call
		return fixedResponse(), nil
	})
	remembered := 0
	h.app.env.remember = func([]string) { remembered++ }

	result, err := h.call(remote.Request{Command: remote.CommandEnv})
	if err != nil {
		tt.Fatal(err)
	}
	listed := result.([]remote.Environment)
	if len(listed) != 2 || listed[0].Name != "local" || !listed[0].Active || listed[1].Active ||
		strings.Join(listed[1].Variables, ",") != "BASE_URL,API_TOKEN,PASSWORD" {
		tt.Errorf("env listed %+v", listed)
	}

	result, err = h.call(remote.Request{Command: remote.CommandEnv, Environment: "STAGING"})
	if err != nil {
		tt.Fatal(err)
	}
	if listed := result.([]remote.Environment); listed[0].Active || !listed[1].Active {
		tt.Errorf("after switching, env listed %+v", listed)
	}
	if e := h.send(remote.Request{Ref: "users/list-users"}); e.Environment != "staging" ||
		sent.Variables["BASE_URL"] != "https://staging.example.com/api" {
		tt.Errorf("sent in %q with BASE_URL %q", e.Environment, sent.Variables["BASE_URL"])
	}

	if _, err := h.call(remote.Request{Command: remote.CommandEnv, NoEnvironment: true}); err != nil {
		tt.Fatal(err)
	}
	if name := h.app.env.active.Peek().Name; name != "" {
		tt.Errorf("--none left %q active", name)
	}
	if _, err := h.call(remote.Request{Command: remote.CommandEnv, Environment: "prod"}); err == nil {
		tt.Error("switched to an environment that doesn't exist")
	}
	if remembered != 0 {
		tt.Error("a remote switch was remembered for the next launch")
	}
}

// savedStore records what's saved.
type savedStore struct{ saved []model.Request }

func (s *savedStore) Save(r model.Request) error { s.saved = append(s.saved, r); return nil }
func (s *savedStore) Delete(string) error        { return nil }

func TestRemoteSavesTheActiveTab(tt *testing.T) {
	h := newRemoteHarness(tt)
	store := &savedStore{}
	h.app.store = store
	save := func(req remote.Request) (remote.Shown, error) {
		req.Command = remote.CommandSave
		result, err := h.call(req)
		if err != nil {
			return remote.Shown{}, err
		}
		return result.(remote.Shown), nil
	}

	if _, err := h.call(remote.Request{Command: remote.CommandOpen, Curl: "curl https://example.com/health"}); err != nil {
		tt.Fatal(err)
	}
	if _, err := save(remote.Request{}); err == nil {
		tt.Error("saved a request with no name or file")
	}
	shown, err := save(remote.Request{File: "ops/health", Name: "Health"})
	if err != nil {
		tt.Fatal(err)
	}
	s := h.app.current()
	if shown.Tab.File != "ops/health.posting.yaml" || shown.Tab.Title != "Health" || shown.Tab.Dirty ||
		s.file.Peek() != "ops/health.posting.yaml" || s.name.GetText() != "Health" {
		tt.Errorf("saved as %+v", shown.Tab)
	}
	if len(store.saved) != 1 || store.saved[0].URL != "https://example.com/health" {
		tt.Errorf("store got %+v", store.saved)
	}
	if _, err := h.app.findSaved("ops/health"); err != nil {
		tt.Errorf("the saved request isn't in the collection: %v", err)
	}

	// Saving again writes in place; unsaved requests now get a tab of their own.
	s.url.SetText("https://example.com/healthz")
	if shown, err := save(remote.Request{}); err != nil || shown.Tab.File != "ops/health.posting.yaml" || len(store.saved) != 2 {
		tt.Errorf("saving in place gave %+v, %v", shown.Tab, err)
	}
	tabs := len(h.app.sessions.Peek())
	h.send(remote.Request{Curl: "curl https://example.com/other"})
	if len(h.app.sessions.Peek()) != tabs+1 {
		tt.Error("an unsaved request replaced the saved tab")
	}

	// Named after the request when no file is given.
	if shown, err := save(remote.Request{Name: "Other thing"}); err != nil || shown.Tab.File != "other-thing.posting.yaml" {
		tt.Errorf("saving by name gave %+v, %v", shown.Tab, err)
	}

	for _, file := range []string{"ops/health", "users/list-users.posting.yaml", "../escape", "/abs/file", "c:/x"} {
		if _, err := save(remote.Request{File: file}); err == nil {
			tt.Errorf("saved over or outside the collection as %q", file)
		}
	}
}
