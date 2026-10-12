package ui

import (
	"sync"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

// memoryHistory records saves for the test.
type memoryHistory struct {
	mu      sync.Mutex
	entries []model.HistoryEntry
	saves   int
}

func (m *memoryHistory) Load() ([]model.HistoryEntry, error) { return m.entries, nil }

func (m *memoryHistory) Save(entries []model.HistoryEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = entries
	m.saves++
	return nil
}

func (m *memoryHistory) snapshot() []model.HistoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries
}

func TestHistoryPersists(tt *testing.T) {
	stored := &memoryHistory{entries: []model.HistoryEntry{
		{ID: 7, Request: sampleRequest(tt, "List users"), Response: fixedResponse(), SentAt: time.Now()},
	}}
	app := New(Config{Collection: model.SampleCollection(), History: stored, UserHost: "user@host"})
	if got := app.history.Peek(); len(got) != 1 || got[0].ID != 7 {
		tt.Fatalf("history not loaded: %+v", got)
	}

	app.nextHistoryID++
	entry := sentEntry(app.nextHistoryID, sampleRequest(tt, "Get user"), fixedResponse(), time.Now())
	if entry.ID != 8 {
		tt.Fatalf("new entries should continue from the stored IDs, got %d", entry.ID)
	}
	app.setHistory(append([]model.HistoryEntry{entry}, app.history.Peek()...))
	waitFor(tt, func() bool { return len(stored.snapshot()) == 2 })

	app.clearHistory()
	waitFor(tt, func() bool { return len(stored.snapshot()) == 0 })
}
