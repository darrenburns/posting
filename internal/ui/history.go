package ui

import (
	"errors"
	"fmt"
	"sync"

	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
)

// HistoryStore applies individual changes to the latest persisted history.
type HistoryStore interface {
	Load() ([]model.HistoryEntry, error)
	Append(model.HistoryEntry) ([]model.HistoryEntry, error)
	Delete([]int64) ([]model.HistoryEntry, error)
	Clear() ([]model.HistoryEntry, error)
}

type historyChange func(HistoryStore) ([]model.HistoryEntry, error)

type historyJob struct {
	apply    historyChange
	complete func([]model.HistoryEntry, error)
}

// historySaver preserves the order of every operation without blocking the UI.
type historySaver struct {
	mu      sync.Mutex
	ready   *sync.Cond
	pending []historyJob
	closed  bool
	done    chan struct{}
	err     error
}

func newHistorySaver(store HistoryStore) *historySaver {
	s := &historySaver{done: make(chan struct{})}
	s.ready = sync.NewCond(&s.mu)
	go func() {
		defer close(s.done)
		for {
			s.mu.Lock()
			for len(s.pending) == 0 && !s.closed {
				s.ready.Wait()
			}
			if len(s.pending) == 0 {
				s.mu.Unlock()
				return
			}
			job := s.pending[0]
			s.pending[0] = historyJob{}
			s.pending = s.pending[1:]
			s.mu.Unlock()
			entries, err := job.apply(store)
			s.mu.Lock()
			if err != nil && s.err == nil {
				s.err = err
			}
			closed := s.closed
			s.mu.Unlock()
			if !closed {
				job.complete(entries, err)
			}
		}
	}()
	return s
}

func (s *historySaver) enqueue(job historyJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("history is closed")
	}
	s.pending = append(s.pending, job)
	s.ready.Signal()
	return nil
}

func (s *historySaver) close() error {
	s.mu.Lock()
	s.closed = true
	s.ready.Broadcast()
	s.mu.Unlock()
	<-s.done
	return s.err
}

// Close drains history changes before exiting. It can be called more than once.
// Draining does not wait for the UI to process completion callbacks.
func (a *App) Close() error {
	if a.historyStore == nil {
		return nil
	}
	if err := a.historyStore.close(); err != nil {
		return fmt.Errorf("couldn't save history: %w", err)
	}
	return nil
}

func (a *App) queueHistory(change historyChange, after func()) {
	dispatch := a.dispatch
	err := a.historyStore.enqueue(historyJob{
		apply: change,
		complete: func(entries []model.HistoryEntry, err error) {
			if err == nil {
				for i := range entries {
					entries[i].Status = model.StatusOf(entries[i].Request, entries[i].Response)
				}
			}
			dispatch(func() {
				if err != nil {
					a.notify("Couldn't save history: "+err.Error(), toastError)
					return
				}
				a.setHistory(entries)
				if after != nil {
					after()
				}
			})
		},
	})
	if err != nil {
		a.notify("Couldn't save history: "+err.Error(), toastError)
	}
}

func (a *App) appendHistory(entry model.HistoryEntry) {
	if a.historyStore != nil {
		a.queueHistory(func(store HistoryStore) ([]model.HistoryEntry, error) { return store.Append(entry) }, nil)
		return
	}
	a.nextHistoryID++
	entry.ID = a.nextHistoryID
	a.setHistory(append([]model.HistoryEntry{entry}, a.history.Peek()...))
}

// setHistory updates the displayed history and preserves selection by ID.
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
}
