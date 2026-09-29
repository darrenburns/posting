package ui

import (
	"github.com/darrenburns/posting/internal/history"
	"github.com/darrenburns/posting/internal/model"
)

// HistoryStore keeps history between runs.
type HistoryStore interface {
	Load() ([]model.HistoryEntry, error)
	Save([]model.HistoryEntry) error
}

// historySaver writes history in the background, one save at a time and
// always the latest, so a slow disk never holds up the UI.
type historySaver struct {
	pending chan []model.HistoryEntry
}

func newHistorySaver(store HistoryStore) *historySaver {
	s := &historySaver{pending: make(chan []model.HistoryEntry, 1)}
	go func() {
		for entries := range s.pending {
			_ = store.Save(entries)
		}
	}()
	return s
}

// save queues entries, replacing any save still waiting.
func (s *historySaver) save(entries []model.HistoryEntry) {
	for {
		select {
		case s.pending <- entries:
			return
		default:
			select {
			case <-s.pending:
			default:
			}
		}
	}
}

// setHistory replaces the history, newest first, within the limits, and
// saves it when history is kept on disk.
func (a *App) setHistory(entries []model.HistoryEntry) {
	entries = history.Trim(entries)
	a.history.Set(entries)
	a.historyList.SetItems(entries)
	if a.historyStore != nil {
		a.historyStore.save(append([]model.HistoryEntry(nil), entries...))
	}
}
