// Package history keeps sent requests and their responses on disk, one file
// per collection, outside the collection itself so it never ends up in
// version control.
package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/darrenburns/posting/internal/model"
)

// Limits match Posting 2: the newest 100 exchanges, within 50 MiB.
const (
	MaxEntries = 100
	MaxBytes   = 50 << 20
)

// Store is the history of one collection.
type Store struct {
	path string
	mu   sync.Mutex
	// unreadable are the entries the last Load couldn't decode, such as a
	// kind of request from a newer Posting. Save writes them back as they
	// were, after the others, so this build never loses them.
	unreadable []json.RawMessage
}

// ForCollection returns the store for the collection at root, kept in dir.
func ForCollection(dir, root string) *Store {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(abs))
	return &Store{path: filepath.Join(dir, hex.EncodeToString(sum[:])+".json")}
}

// Path is where the history is stored.
func (s *Store) Path() string { return s.path }

// file is the history file. Entries are decoded one by one, so one that
// can't be doesn't lose the rest.
type file struct {
	Version int               `json:"version"`
	Entries []json.RawMessage `json:"entries"`
}

// Load reads the history, newest first. A store that doesn't exist yet is
// empty. Entries that can't be decoded are left out, and kept for Save.
func (s *Store) Load() ([]model.HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	var entries []model.HistoryEntry
	s.unreadable = nil
	for _, raw := range f.Entries {
		var entry model.HistoryEntry
		if json.Unmarshal(raw, &entry) != nil {
			s.unreadable = append(s.unreadable, raw)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Save replaces the history with entries (newest first), keeping only the
// newest that fit the limits, then the entries Load couldn't decode. The
// file is private to the user, since responses often contain credentials.
func (s *Store) Save(entries []model.HistoryEntry) error {
	var raw []json.RawMessage
	for _, entry := range Trim(entries) {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		raw = append(raw, data)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(file{Version: 1, Entries: append(raw, s.unreadable...)})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".history-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// Trim keeps the newest entries that fit within MaxEntries and MaxBytes.
// An exchange that cannot fit on its own is skipped without evicting history.
func Trim(entries []model.HistoryEntry) []model.HistoryEntry {
	total := 0
	var kept []model.HistoryEntry
	for _, e := range entries {
		if len(kept) >= MaxEntries {
			break
		}
		size := e.Request.PayloadSize()
		if e.Response != nil {
			size += len(e.Response.Body)
		}
		if size > MaxBytes {
			continue
		}
		if total+size > MaxBytes {
			break
		}
		total += size
		kept = append(kept, e)
	}
	return kept
}
