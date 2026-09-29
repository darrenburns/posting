package collection

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

// Store persists requests. The UI saves and deletes through it, so it never
// touches the filesystem itself.
type Store interface {
	// Save writes req to req.File, creating folders as needed.
	Save(req model.Request) error
	// Delete removes the request stored at file.
	Delete(file string) error
}

// Dir is a collection stored in a directory.
type Dir struct {
	Root string
}

// LoadError records a request file that couldn't be read. The rest of the
// collection still loads.
type LoadError struct {
	File string
	Err  error
}

func (e LoadError) Error() string { return e.File + ": " + e.Err.Error() }

// Load reads every request file under the directory. Folders without request
// files are still included, so newly created folders show up.
func (d Dir) Load() (*model.Collection, []LoadError) {
	root := &model.Collection{Name: filepath.Base(d.Root)}
	var problems []LoadError
	err := filepath.WalkDir(d.Root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			if p == d.Root {
				return err
			}
			problems = append(problems, LoadError{File: p, Err: err})
			return nil
		}
		rel, relErr := filepath.Rel(d.Root, p)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__pycache__" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), FileSuffix) {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			problems = append(problems, LoadError{File: rel, Err: readErr})
			return nil
		}
		req, parseErr := ParseRequest(data, rel)
		if parseErr != nil {
			problems = append(problems, LoadError{File: rel, Err: parseErr})
			return nil
		}
		folder := EnsureFolder(root, path.Dir(rel))
		folder.Requests = append(folder.Requests, req)
		return nil
	})
	if err != nil {
		problems = append(problems, LoadError{File: d.Root, Err: err})
	}
	pruneEmpty(root)
	root.Sort()
	sort.Slice(problems, func(i, j int) bool { return problems[i].File < problems[j].File })
	return root, problems
}

// pruneEmpty removes folders with no requests anywhere beneath them, such as
// a collection's scripts folder.
func pruneEmpty(c *model.Collection) bool {
	kept := c.Children[:0]
	for _, child := range c.Children {
		if !pruneEmpty(child) {
			kept = append(kept, child)
		}
	}
	c.Children = kept
	return len(c.Children) == 0 && len(c.Requests) == 0
}

// Save writes req to its file, replacing any existing file atomically.
func (d Dir) Save(req model.Request) error {
	target, err := d.resolve(req.File)
	if err != nil {
		return err
	}
	data, err := MarshalRequest(req)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".posting-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// Delete removes a request file. A file that is already gone is not an error.
func (d Dir) Delete(file string) error {
	target, err := d.resolve(file)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// resolve turns a collection-relative path into a path on disk, refusing
// anything that would land outside the collection.
func (d Dir) resolve(file string) (string, error) {
	clean := path.Clean("/" + file)[1:]
	if file == "" || clean != file || !strings.HasSuffix(file, FileSuffix) {
		return "", fmt.Errorf("invalid request path %q", file)
	}
	return filepath.Join(d.Root, filepath.FromSlash(clean)), nil
}

// EnsureFolder returns the folder at the collection-relative path, creating
// any folders missing along the way.
func EnsureFolder(root *model.Collection, folderPath string) *model.Collection {
	if folderPath == "" || folderPath == "." {
		return root
	}
	folder := root
	built := ""
	for _, part := range strings.Split(folderPath, "/") {
		if built == "" {
			built = part
		} else {
			built += "/" + part
		}
		var next *model.Collection
		for _, child := range folder.Children {
			if child.Name == part {
				next = child
			}
		}
		if next == nil {
			next = &model.Collection{Name: part, Path: built}
			folder.Children = append(folder.Children, next)
		}
		folder = next
	}
	return folder
}

// Memory is a Store that keeps nothing, for tests and the built-in sample.
type Memory struct{}

func (Memory) Save(model.Request) error { return nil }
func (Memory) Delete(string) error      { return nil }
