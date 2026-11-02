package ui

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
)

type memoryHistory struct {
	mu      sync.Mutex
	entries []model.HistoryEntry
	nextID  int64
	before  func()
	failure error
}

func (m *memoryHistory) Load() ([]model.HistoryEntry, error) { return m.snapshot(), nil }

func (m *memoryHistory) change(apply func()) ([]model.HistoryEntry, error) {
	if m.before != nil {
		m.before()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return nil, m.failure
	}
	for _, entry := range m.entries {
		m.nextID = max(m.nextID, entry.ID)
	}
	apply()
	m.entries = history.Trim(m.entries)
	return slices.Clone(m.entries), nil
}

func (m *memoryHistory) Append(entry model.HistoryEntry) ([]model.HistoryEntry, error) {
	return m.change(func() {
		m.nextID++
		entry.ID = m.nextID
		m.entries = append([]model.HistoryEntry{entry}, m.entries...)
	})
}

func (m *memoryHistory) Delete(ids []int64) ([]model.HistoryEntry, error) {
	return m.change(func() {
		m.entries = slices.DeleteFunc(m.entries, func(entry model.HistoryEntry) bool { return slices.Contains(ids, entry.ID) })
	})
}

func (m *memoryHistory) Clear() ([]model.HistoryEntry, error) {
	return m.change(func() { m.entries = nil })
}

func (m *memoryHistory) snapshot() []model.HistoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.entries)
}

func TestHistoryPersists(tt *testing.T) {
	stored := &memoryHistory{entries: []model.HistoryEntry{
		{ID: 7, Request: sampleRequest(tt, "List users"), Response: fixedResponse(), SentAt: time.Now()},
	}}
	app := New(Config{Collection: model.SampleCollection(), History: stored, UserHost: "user@host"})
	defer app.Close()
	updates := make(chan func(), 8)
	app.dispatch = func(fn func()) { updates <- fn }
	if got := app.history.Peek(); len(got) != 1 || got[0].ID != 7 {
		tt.Fatalf("history not loaded: %+v", got)
	}
	app.appendHistory(sentEntry(0, sampleRequest(tt, "Get user"), fixedResponse(), time.Now()))
	historyUpdate(tt, updates)
	if got := app.history.Peek(); len(got) != 2 || got[0].ID != 8 {
		tt.Fatalf("history did not receive committed IDs: %+v", got)
	}
	app.clearHistory()
	historyUpdate(tt, updates)
	if len(stored.snapshot()) != 0 || len(app.history.Peek()) != 0 {
		tt.Fatal("clear did not persist and update history")
	}
}

func historyUpdate(t *testing.T, updates <-chan func()) {
	t.Helper()
	select {
	case update := <-updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("history update was not delivered")
	}
}

func TestHistoryQueuePreservesOperationsAndDrainsWithoutUI(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	stored := &memoryHistory{before: func() { once.Do(func() { close(entered); <-release }) }}
	app := New(Config{Collection: model.SampleCollection(), History: stored, UserHost: "user@host"})
	updates := make(chan func(), 16)
	app.dispatch = func(fn func()) { updates <- fn }
	appendEntry := func(name string) {
		req := model.NewRequest()
		req.Name = name
		app.appendHistory(model.HistoryEntry{Request: req})
	}
	appendEntry("before-clear")
	<-entered
	app.clearHistory()
	appendEntry("delete-this")
	app.deleteHistory([]model.HistoryEntry{{ID: 2}})
	appendEntry("keep-this")
	if len(app.history.Peek()) != 0 {
		t.Fatal("uncommitted changes reached the display")
	}
	closed := make(chan error, 1)
	go func() { closed <- app.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned before a pending write: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close waited for the UI to process callbacks")
	}
	got := stored.snapshot()
	if len(got) != 1 || got[0].ID != 3 || got[0].Request.Name != "keep-this" {
		t.Fatalf("queued operations were lost or reordered: %+v", got)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestHistoryFailurePreservesDisplayAndReportsError(t *testing.T) {
	original := model.HistoryEntry{ID: 1, Request: model.NewRequest()}
	stored := &memoryHistory{entries: []model.HistoryEntry{original}, failure: errors.New("disk is full")}
	app := New(Config{Collection: model.SampleCollection(), History: stored, UserHost: "user@host"})
	updates := make(chan func(), 2)
	app.dispatch = func(fn func()) { updates <- fn }
	app.clearHistory()
	historyUpdate(t, updates)
	if got := app.history.Peek(); len(got) != 1 || got[0].ID != 1 {
		t.Fatal("failed clear changed displayed history")
	}
	assertToast(t, app, "disk is full")
	if err := app.Close(); err == nil || !strings.Contains(err.Error(), "disk is full") {
		t.Fatalf("Close did not report persistence error: %v", err)
	}
}

func TestSetHistoryOnlyChangesTheDisplay(t *testing.T) {
	store := history.ForCollection(t.TempDir(), "/shared")
	app := New(Config{Collection: model.SampleCollection(), History: store, UserHost: "user@host"})
	app.setHistory([]model.HistoryEntry{{ID: 99}})
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || len(got) != 0 {
		t.Fatalf("display update wrote history: %+v %v", got, err)
	}
}

func TestHistoryClosePersistsRealStore(t *testing.T) {
	dir := t.TempDir()
	store := history.ForCollection(dir, "/shared")
	app := New(Config{Collection: model.SampleCollection(), History: store, UserHost: "user@host"})
	app.dispatch = func(func()) {}
	for _, name := range []string{"first", "second", "third"} {
		req := model.NewRequest()
		req.Name = name
		app.appendHistory(model.HistoryEntry{Request: req})
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := history.ForCollection(dir, "/shared").Load()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range loaded {
		names = append(names, entry.Request.Name)
	}
	if !reflect.DeepEqual(names, []string{"third", "second", "first"}) {
		t.Fatalf("Close did not persist all queued sends: %v", names)
	}
}
