// Package history keeps sent requests and their responses outside the collection.
package history

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/darrenburns/posting/v3/internal/model"
)

const (
	MaxEntries = 100
	MaxBytes   = 50 << 20
)

// Store applies history changes against the latest file under a process lock.
type Store struct{ path string }

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

type file struct {
	Version int               `json:"version"`
	NextID  int64             `json:"next_id"`
	Entries []json.RawMessage `json:"entries"`
}

func (s *Store) read() (file, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return file{Version: 1, NextID: 1}, nil
	}
	if err != nil {
		return file{}, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return file{}, err
	}
	f.NextID = max(f.NextID, 1)
	for _, raw := range f.Entries {
		id := rawID(raw)
		if id == math.MaxInt64 {
			f.NextID = math.MaxInt64
		} else {
			f.NextID = max(f.NextID, id+1)
		}
	}
	return f, nil
}

func rawID(raw json.RawMessage) int64 {
	var header struct{ ID int64 }
	_ = json.Unmarshal(raw, &header)
	return header.ID
}

func (f file) decoded() []model.HistoryEntry {
	var entries []model.HistoryEntry
	for _, raw := range f.Entries {
		var entry model.HistoryEntry
		if json.Unmarshal(raw, &entry) == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Load reads the latest committed history. Unknown request kinds stay on disk.
func (s *Store) Load() ([]model.HistoryEntry, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	defer unlockFile(lock)
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.decoded(), nil
}

// Append allocates an ID and retains the newest exchanges within the limits.
// The caller's ID is ignored. IDs are never reused after deletion or clearing.
func (s *Store) Append(entry model.HistoryEntry) ([]model.HistoryEntry, error) {
	return s.change(func(f *file) error {
		if f.NextID == math.MaxInt64 {
			return errors.New("history entry IDs exhausted")
		}
		entry.ID = f.NextID
		f.NextID++
		if entrySize(entry) > MaxBytes {
			return nil
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		f.Entries = append([]json.RawMessage{raw}, f.Entries...)
		return nil
	})
}

// Delete removes only the named IDs, including entries this build cannot decode.
func (s *Store) Delete(ids []int64) ([]model.HistoryEntry, error) {
	return s.change(func(f *file) error {
		f.Entries = slices.DeleteFunc(f.Entries, func(raw json.RawMessage) bool {
			return slices.Contains(ids, rawID(raw))
		})
		return nil
	})
}

// Clear removes all entries present when the operation acquires the lock.
func (s *Store) Clear() ([]model.HistoryEntry, error) {
	return s.change(func(f *file) error { f.Entries = nil; return nil })
}

func (s *Store) lock() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	// The sidecar stays in place: replacing it would allow two independent locks.
	if err := lockFile(lock); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

func (s *Store) change(apply func(*file) error) ([]model.HistoryEntry, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	defer unlockFile(lock)
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	if err := apply(&f); err != nil {
		return nil, err
	}
	f.trim()
	if err := s.write(f); err != nil {
		return nil, err
	}
	return f.decoded(), nil
}

func (f *file) trim() {
	total := 0
	kept := make([]json.RawMessage, 0, min(len(f.Entries), MaxEntries))
	for _, raw := range f.Entries {
		if len(kept) == MaxEntries {
			break
		}
		size := len(raw)
		var entry model.HistoryEntry
		if json.Unmarshal(raw, &entry) == nil {
			size = entrySize(entry)
		}
		if size > MaxBytes {
			continue
		}
		if total+size > MaxBytes {
			break
		}
		total += size
		kept = append(kept, raw)
	}
	f.Entries = kept
}

func (s *Store) write(f file) error {
	var data bytes.Buffer
	fmt.Fprintf(&data, `{"version":%d,"next_id":%d,"entries":[`, f.Version, f.NextID)
	for i, raw := range f.Entries {
		if i > 0 {
			data.WriteByte(',')
		}
		data.Write(raw)
	}
	data.WriteString("]}")
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".history-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func entrySize(entry model.HistoryEntry) int {
	size := entry.Request.PayloadSize()
	if entry.Response != nil {
		size += len(entry.Response.Body)
		for _, event := range entry.Response.Events {
			size += len(event.Body) + len(event.Title) + len(event.ContentType) + 128
		}
	}
	return size
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
		size := entrySize(e)
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
