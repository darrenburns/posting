package importing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

// Written records the actual collection-relative filenames, including any
// suffixes needed to avoid replacing existing files.
type Written struct {
	Files       []string
	Environment string
}

// DirectoryName turns a source collection name into a portable directory name.
func DirectoryName(name string) string { return safePart(name) }

// Write saves an import into dir without overwriting existing files. All data
// is encoded before writing, and newly created files are removed on failure.
// os.Root confines filesystem operations even when a destination contains
// symlinks. Empty directories may remain after an unsuccessful import.
func Write(result Result, dir string) (written Written, err error) {
	type entry struct {
		name        string
		data        []byte
		environment bool
	}
	entries := make([]entry, 0, len(result.Requests)+1)
	for _, req := range result.Requests {
		name := req.File
		if name == "" {
			name = req.DisplayName()
		}
		name = strings.TrimSuffix(name, collection.FileSuffix)
		name = safePath(name) + collection.FileSuffix
		data, encodeErr := collection.MarshalRequest(req)
		if encodeErr != nil {
			return Written{}, fmt.Errorf("encode %q: %w", req.DisplayName(), encodeErr)
		}
		// A saved request must be readable by Posting (e.g. an unsupported method
		// should fail before creating any output files).
		if _, parseErr := collection.ParseRequest(data, name); parseErr != nil {
			return Written{}, fmt.Errorf("encode %q: %w", req.DisplayName(), parseErr)
		}
		entries = append(entries, entry{name: name, data: data})
	}
	if len(result.Variables) > 0 {
		data, encodeErr := environmentData(result.Variables)
		if encodeErr != nil {
			return Written{}, encodeErr
		}
		entries = append(entries, entry{name: "imported.env", data: data, environment: true})
	}
	if len(entries) == 0 {
		return Written{}, fmt.Errorf("source contains no importable requests or variables")
	}
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return Written{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Written{}, err
	}
	defer root.Close()
	var created []string
	defer func() {
		if err != nil {
			for i := len(created) - 1; i >= 0; i-- {
				if removeErr := root.Remove(created[i]); removeErr != nil {
					err = errors.Join(err, fmt.Errorf("remove incomplete import %q: %w", created[i], removeErr))
				}
			}
			written = Written{}
		}
	}()
	used := map[string]bool{}
	for _, item := range entries {
		if err = root.MkdirAll(filepath.Dir(item.name), 0o755); err != nil {
			return written, fmt.Errorf("create import folder: %w", err)
		}
		var file *os.File
		var name string
		for n := 1; ; n++ {
			name = numberedName(item.name, n)
			if used[strings.ToLower(name)] {
				continue
			}
			file, err = root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			if err != nil {
				return written, fmt.Errorf("create %q: %w", name, err)
			}
			break
		}
		created = append(created, name)
		used[strings.ToLower(name)] = true
		_, writeErr := file.Write(item.data)
		closeErr := file.Close()
		if err = errors.Join(writeErr, closeErr); err != nil {
			return written, fmt.Errorf("write %q: %w", name, err)
		}
		if item.environment {
			written.Environment = name
		} else {
			written.Files = append(written.Files, name)
		}
	}
	return written, nil
}

func numberedName(name string, n int) string {
	if n == 1 {
		return name
	}
	suffix := filepath.Ext(name)
	if strings.HasSuffix(name, collection.FileSuffix) {
		suffix = collection.FileSuffix
	}
	return strings.TrimSuffix(name, suffix) + "-" + strconv.Itoa(n) + suffix
}

func safePath(name string) string {
	// Treat both platform separators as path separators, independently of the
	// OS on which an imported collection was created.
	parts := strings.Split(strings.ReplaceAll(name, `\`, "/"), "/")
	for i := range parts {
		parts[i] = safePart(parts[i])
	}
	return filepath.Join(parts...)
}

func safePart(name string) string {
	var out strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			r = '-'
		}
		// Leave room for collision suffixes and .posting.yaml on all platforms.
		if out.Len()+len(string(r)) > 120 {
			break
		}
		out.WriteRune(r)
	}
	name = strings.Trim(out.String(), " .\t")
	if name == "" {
		name = "untitled"
	}
	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name
}

func environmentData(variables []model.Variable) ([]byte, error) {
	var out strings.Builder
	out.WriteString("# Imported collection defaults. Load with posting -e <this file>.\n")
	for _, v := range variables {
		refs := model.FindVariables("${" + v.Name + "}")
		if len(refs) != 1 || refs[0].Name != v.Name {
			return nil, fmt.Errorf("cannot save environment variable named %q", v.Name)
		}
		// Single quotes preserve literal dollar expressions and host-independent
		// values. Backslashes and quotes use dotenv's single-quoted escapes.
		value := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v.Value)
		out.WriteString(v.Name + "='" + value + "'\n")
	}
	return []byte(out.String()), nil
}
