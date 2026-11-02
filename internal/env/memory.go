package env

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// Memory remembers the environment last switched to in each collection, so
// the next launch starts in it. It's one JSON file keyed by collection
// directory.
type Memory struct {
	File string
}

// remembered is one collection's environment. A named environment is kept
// by name and folder and rebuilt when recalled, so layers added since (a new
// staging.local.env, say) are included. Any other set of files is kept, and
// recalled, exactly.
type remembered struct {
	Name  string   `json:"name,omitempty"`
	Dir   string   `json:"dir,omitempty"`
	Files []string `json:"files,omitempty"`
}

func (m Memory) read() (map[string]remembered, error) {
	data, err := os.ReadFile(m.File)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]remembered{}, nil
	}
	if err != nil {
		return nil, err
	}
	saved := map[string]remembered{}
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	return saved, nil
}

// Recall is the environment remembered for a collection, or nil if there is
// none or it has since gone: a named environment no longer in its folder, or
// a file missing from a set of files.
func (m Memory) Recall(collection string) []string {
	saved, err := m.read()
	if err != nil {
		return nil
	}
	entry := saved[collection]
	if entry.Name != "" {
		return Stack(entry.Dir, entry.Name)
	}
	for _, file := range entry.Files {
		if info, err := os.Stat(file); err != nil || info.IsDir() {
			return nil
		}
	}
	return entry.Files
}

// Remember records a collection's environment. No files forgets it, so the
// next launch starts in the default environment.
func (m Memory) Remember(collection string, files []string) error {
	saved, err := m.read()
	if err != nil {
		saved = map[string]remembered{}
	}
	if len(files) == 0 {
		delete(saved, collection)
	} else {
		saved[collection] = identify(files)
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

// identify names files if they're exactly a named environment's stack.
func identify(files []string) remembered {
	dir := filepath.Dir(files[0])
	for _, file := range files[1:] {
		if filepath.Dir(file) != dir {
			return remembered{Files: files}
		}
	}
	name := Name(files)
	if slices.Equal(Stack(dir, name), files) {
		return remembered{Name: name, Dir: dir}
	}
	return remembered{Files: files}
}
