package env

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Memory remembers the environment last switched to in each collection, so
// the next launch starts in it. It's one JSON file mapping a collection
// directory to its environment's files.
type Memory struct {
	File string
}

func (m Memory) read() (map[string][]string, error) {
	data, err := os.ReadFile(m.File)
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	saved := map[string][]string{}
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	return saved, nil
}

// Recall is the environment remembered for a collection, or nil if there is
// none or any of its files has since gone.
func (m Memory) Recall(collection string) []string {
	saved, err := m.read()
	if err != nil {
		return nil
	}
	files := saved[collection]
	for _, file := range files {
		if info, err := os.Stat(file); err != nil || info.IsDir() {
			return nil
		}
	}
	return files
}

// Remember records a collection's environment. No files forgets it, so the
// next launch starts in the default environment.
func (m Memory) Remember(collection string, files []string) error {
	saved, err := m.read()
	if err != nil {
		saved = map[string][]string{}
	}
	if len(files) == 0 {
		delete(saved, collection)
	} else {
		saved[collection] = files
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.File), 0o755); err != nil {
		return err
	}
	tmp := m.File + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.File)
}
