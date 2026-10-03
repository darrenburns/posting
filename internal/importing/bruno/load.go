package bruno

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

// Load imports a .bru file or a collection directory containing bruno.json.
// It never executes scripts, reads upload references, or follows symlinks.
func Load(path string) (importing.Result, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return importing.Result{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return importing.Result{}, fmt.Errorf("Bruno input must not be a symlink")
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return importing.Result{}, fmt.Errorf("Bruno input must be a regular file")
		}
		data, err := readBounded(path)
		if err != nil {
			return importing.Result{}, err
		}
		r, err := Parse(data)
		if err == nil && len(r.Requests) > 0 {
			r.Requests[0].File = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".posting.yaml"
		}
		return r, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return importing.Result{}, err
	}
	defer root.Close()
	result := importing.Result{Name: filepath.Base(filepath.Clean(path)), DeferredVariables: true}
	count, total, visited := 0, 0, 0
	materialized := 0
	read := func(name string) ([]byte, error) {
		st, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", name)
		}
		count++
		if count > 10000 {
			return nil, fmt.Errorf("Bruno collection exceeds 10000 files")
		}
		f, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxFileSize {
			return nil, fmt.Errorf("%s exceeds 16 MiB", name)
		}
		total += len(data)
		if total > 128<<20 {
			return nil, fmt.Errorf("Bruno collection exceeds 128 MiB")
		}
		return data, nil
	}
	config, err := read("bruno.json")
	if err != nil {
		return importing.Result{}, fmt.Errorf("read bruno.json: %w", err)
	}
	var cfg struct {
		Name   string   `json:"name"`
		Type   string   `json:"type"`
		Ignore []string `json:"ignore"`
		// protoFiles only lists files a request may pick as its protoPath.
		Protobuf struct {
			ImportPaths []struct {
				Path    string `json:"path"`
				Enabled bool   `json:"enabled"`
			} `json:"importPaths"`
		} `json:"protobuf"`
	}
	if err = json.Unmarshal(config, &cfg); err != nil {
		return importing.Result{}, fmt.Errorf("bruno.json: %w", err)
	}
	if cfg.Name != "" {
		result.Name = cfg.Name
	}
	if cfg.Type != "" && cfg.Type != "collection" {
		return importing.Result{}, fmt.Errorf("bruno.json is not a collection")
	}
	if len(cfg.Ignore) > 0 {
		warn(&result, "bruno.json ignore patterns are not interpreted; hidden directories and node_modules are excluded")
	}
	readDir := func(dir string) ([]fs.DirEntry, error) {
		f, err := root.Open(dir)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var entries []fs.DirEntry
		for {
			batch, e := f.ReadDir(256)
			visited += len(batch)
			if visited > 20000 {
				return nil, fmt.Errorf("Bruno collection exceeds 20000 directory entries")
			}
			entries = append(entries, batch...)
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
		}
		// ReadDir on os.File is directory order; sort for deterministic output.
		sortEntries(entries)
		return entries, nil
	}
	loadEnvironments := func(dir string) error {
		entries, err := readDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			n := entry.Name()
			rel := filepath.Join(dir, n)
			switch {
			case strings.HasPrefix(n, "."):
				continue
			case entry.Type()&os.ModeSymlink != 0:
				warn(&result, "skipped symlink "+rel)
				continue
			case entry.IsDir() || filepath.Ext(n) != ".bru":
				warn(&result, "skipped "+rel+"; Bruno environments are .bru files")
				continue
			}
			data, err := read(rel)
			if err != nil {
				return err
			}
			doc, err := parseDocument(data)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			e, err := environment(doc, strings.TrimSuffix(n, ".bru"), rel, &result)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			result.Environments = append(result.Environments, e)
		}
		return nil
	}
	var walk func(string, scope, int) error
	walk = func(dir string, parent scope, depth int) error {
		if depth > 64 {
			return fmt.Errorf("Bruno collection exceeds 64 folder levels")
		}
		fileName := "folder.bru"
		if dir == "." {
			fileName = "collection.bru"
		}
		name := filepath.Join(dir, fileName)
		if _, err := root.Lstat(name); err == nil {
			data, e := read(name)
			if e != nil {
				return e
			}
			doc, e := parseDocument(data)
			if e != nil {
				return fmt.Errorf("%s: %w", name, e)
			}
			parent, e = applyScope(doc, parent, &result, dir == ".")
			if e != nil {
				return fmt.Errorf("%s: %w", name, e)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		entries, err := readDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			n := entry.Name()
			if n == "collection.bru" || n == "folder.bru" || strings.HasPrefix(n, ".") || n == "node_modules" {
				continue
			}
			rel := filepath.Join(dir, n)
			if entry.Type()&os.ModeSymlink != 0 {
				warn(&result, "skipped symlink "+rel)
				continue
			}
			if entry.IsDir() {
				if dir == "." && n == "environments" {
					if err := loadEnvironments(n); err != nil {
						return err
					}
					continue
				}
				if err := walk(rel, parent, depth+1); err != nil {
					return err
				}
				continue
			}
			if filepath.Ext(n) != ".bru" {
				continue
			}
			data, e := read(rel)
			if e != nil {
				return e
			}
			doc, e := parseDocument(data)
			if e != nil {
				return fmt.Errorf("%s: %w", rel, e)
			}
			start := len(result.Warnings)
			r, e := convert(doc, parent, &result)
			if e != nil {
				return fmt.Errorf("%s: %w", rel, e)
			}
			for i := start; i < len(result.Warnings); i++ {
				result.Warnings[i] = rel + ": " + result.Warnings[i]
			}
			if r != nil {
				r.File = strings.TrimSuffix(rel, ".bru") + ".posting.yaml"
				// Keep the collection admission budget based on expanded content,
				// even though saved requests now retain their variable templates.
				materialScope := parent
				materialScope.deferred = false
				materialScope.vars = map[string]string{}
				for name, value := range parent.collection {
					materialScope.vars[name] = value
				}
				for name, value := range parent.vars {
					materialScope.vars[name] = value
				}
				material, e := convert(doc, materialScope, &importing.Result{})
				if e != nil {
					return fmt.Errorf("%s: %w", rel, e)
				}
				materialized += max(requestFootprint(*r), requestFootprint(*material))
				if materialized > 128<<20 {
					return fmt.Errorf("materialized Bruno collection exceeds 128 MiB")
				}
				result.Requests = append(result.Requests, *r)
			}
		}
		return nil
	}
	rootScope := emptyScope()
	rootScope.deferred = true
	for _, p := range cfg.Protobuf.ImportPaths {
		if p.Enabled && p.Path != "" {
			rootScope.protoImports = append(rootScope.protoImports, p.Path)
		}
	}
	if err := walk(".", rootScope, 0); err != nil {
		return importing.Result{}, err
	}
	return result, nil
}
func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if len(b) > maxFileSize {
		return nil, fmt.Errorf("Bruno file exceeds 16 MiB")
	}
	return b, err
}
func sortEntries(entries []fs.DirEntry) {
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
}

// Account for scoped values copied into each request, which need not be bounded
// by source file bytes. Include conservative structural overhead for empty rows.
func requestFootprint(r model.Request) int {
	size := 512 + len(r.Name) + len(r.Description) + len(r.URL) + len(r.File) + r.PayloadSize() + len(r.Body.ContentType) + len(r.Auth.Username) + len(r.Auth.Password) + len(r.Auth.Token)
	if r.VariableScope != nil {
		for name, value := range r.VariableScope.Variables {
			size += 64 + len(name) + len(value)
		}
	}
	for _, rows := range [][]model.KeyValue{r.Headers, r.Query, r.PathParams, r.Body.Form} {
		for _, row := range rows {
			size += 64 + len(row.Name) + len(row.Value)
		}
	}
	return size
}
