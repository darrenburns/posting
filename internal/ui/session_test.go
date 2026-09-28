package ui

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

func TestSessionSnapshotRoundTrip(t *testing.T) {
	for _, folder := range model.SampleCollection().Children {
		for _, req := range folder.Requests {
			s := newSession(1, req)
			got := s.Snapshot()
			want := req.Clone()
			if len(want.Query) > 0 {
				// Query rows are written into the URL when a request is opened.
				want.URL = got.URL
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: snapshot differs\n got: %+v\nwant: %+v", req.Name, got, want)
			}
		}
	}
}

func TestSessionURLEditUpdatesQueryAndPathParams(t *testing.T) {
	s := newSession(1, model.NewRequest())
	s.url.SetText("https://api.test/users/:id/posts/:post?page=2&q=a%20b")
	s.urlEdited()

	query := s.query.Values()
	wantQuery := []model.KeyValue{{Name: "page", Value: "2", Enabled: true}, {Name: "q", Value: "a b", Enabled: true}}
	if !reflect.DeepEqual(query, wantQuery) {
		t.Fatalf("query = %+v, want %+v", query, wantQuery)
	}
	var names []string
	for _, p := range s.pathParams.Values() {
		names = append(names, p.Name)
	}
	if !reflect.DeepEqual(names, []string{"id", "post"}) {
		t.Fatalf("path params = %v", names)
	}
	if !s.dirty.Peek() {
		t.Fatal("editing the URL should mark the session dirty")
	}
}

func TestSessionQueryEditRewritesURL(t *testing.T) {
	s := newSession(1, model.NewRequest())
	s.url.SetText("${BASE}/search?old=1#frag")
	s.query.Load([]model.KeyValue{
		{Name: "q", Value: "${TERM}", Enabled: true},
		{Name: "off", Value: "x", Enabled: false},
		{Name: "n", Value: "a&b", Enabled: true},
	})
	s.queryEdited()
	if got, want := s.url.GetText(), "${BASE}/search?q=${TERM}&n=a%26b#frag"; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
	// Re-parsing the URL keeps the disabled row.
	s.urlEdited()
	if got := len(s.query.Values()); got != 3 {
		t.Fatalf("expected 3 query rows after re-parse, got %d", got)
	}
}

func TestKVEditorKeepsOneBlankRow(t *testing.T) {
	e := newKVEditor("test", false, nil)
	if n := len(e.rows.Peek()); n != 1 {
		t.Fatalf("new editor should have one blank row, has %d", n)
	}
	e.rows.Peek()[0].key.SetText("Accept")
	e.changed()
	if n := len(e.rows.Peek()); n != 2 {
		t.Fatalf("typing into the blank row should add another, have %d rows", n)
	}
	e.remove(e.rows.Peek()[0].id)
	if n := len(e.rows.Peek()); n != 1 || len(e.Values()) != 0 {
		t.Fatalf("after removing, want one blank row and no values; got %d rows, %v", n, e.Values())
	}
}

func TestSessionSendLifecycle(t *testing.T) {
	req := model.NewRequest()
	req.URL = "http://api.test/things"
	s := newSession(1, req)

	var done *model.Response
	s.Send(client.Fake{}, nil, func(_ model.Request, resp *model.Response) { done = resp })
	waitFor(t, func() bool { return s.phase.Peek() == exchangeDone })
	if s.response.Peek() == nil || done == nil || s.response.Peek() != done {
		t.Fatal("completed exchange should publish the response")
	}

	failing := client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) {
		return nil, errors.New("connection refused")
	})
	s.Send(failing, nil, nil)
	waitFor(t, func() bool { return s.phase.Peek() == exchangeFailed })
	if err := s.err.Peek(); err == nil || err.Error() != "connection refused" {
		t.Fatalf("err = %v", err)
	}

	s.Send(client.Fake{StageDelay: time.Hour}, nil, nil)
	s.Cancel()
	if s.phase.Peek() != exchangeCancelled {
		t.Fatalf("phase = %v, want cancelled", s.phase.Peek())
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
