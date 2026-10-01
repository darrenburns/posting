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
	old := a.historyList.GetItems()
	selected := a.historyList.Selection.Peek()
	cursor, anchor := a.historyList.CursorIndex.Peek(), a.historyList.GetAnchor()
	indices := make(map[int64]int, len(entries))
	for i, entry := range entries {
		indices[entry.ID] = i
	}
	remap := func(i int) (int, bool) {
		if i < 0 || i >= len(old) {
			return 0, false
		}
		index, ok := indices[old[i].ID]
		return index, ok
	}
	selection := make(map[int]struct{}, len(selected))
	for i := range selected {
		if index, ok := remap(i); ok {
			selection[index] = struct{}{}
		}
	}
	a.history.Set(entries)
	a.historyList.SetItems(entries)
	a.historyList.Selection.Set(selection)
	if index, ok := remap(cursor); ok {
		a.historyList.CursorIndex.Set(index)
	}
	a.historyList.ClearAnchor()
	if index, ok := remap(anchor); ok {
		a.historyList.SetAnchor(index)
	}
	if a.historyStore != nil {
		a.historyStore.save(append([]model.HistoryEntry(nil), entries...))
	}
}
