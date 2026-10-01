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
	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/model"
)

// Written records the actual collection-relative filenames, including any
// suffixes needed to avoid replacing existing files.
type Written struct {
	Files []string
	// Environments lists the environment files written, the base first.
	Environments []WrittenEnvironment
	Warnings     []string
}

// WrittenEnvironment is one environment file. Name is what posting --env
// selects it by.
type WrittenEnvironment struct {
	Name string
	File string
	Base bool
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
		base        bool
	}
	entries := make([]entry, 0, len(result.Requests)+1+len(result.Environments))
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
	files, warnings := layers(result.Variables, result.Environments)
	if len(result.Variables) > 0 {
		data, encodeErr := environmentData(files[0])
		if encodeErr != nil {
			return Written{}, encodeErr
		}
		entries = append(entries, entry{name: env.BaseFile, data: data, environment: true, base: true})
	}
	for i, e := range result.Environments {
		data, encodeErr := environmentData(files[i+1])
		if encodeErr != nil {
			return Written{}, fmt.Errorf("environment %q: %w", e.Name, encodeErr)
		}
		entries = append(entries, entry{name: environmentFile(e.Name), data: data, environment: true})
	}
	if len(entries) == 0 {
		return Written{}, fmt.Errorf("source contains no importable requests, variables or environments")
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
			written.Environments = append(written.Environments, WrittenEnvironment{Name: env.Name([]string{name}), File: name, Base: item.base})
			if item.base && name != env.BaseFile {
				warnings = append(warnings, fmt.Sprintf("%s already exists, so the imported base variables were written to %s; Posting loads that file as an environment named %s, not as the base", env.BaseFile, name, env.Name([]string{name})))
			}
		} else {
			written.Files = append(written.Files, name)
		}
	}
	written.Warnings = warnings
	return written, nil
}

// environmentFile names an environment's file without stepping on the
// layering convention: posting.env is the base, and a .local.env file is a
// companion of another environment rather than an environment of its own.
func environmentFile(name string) string {
	name = safePart(name)
	lower := strings.ToLower(name)
	switch {
	case lower == env.BaseName:
		name += "-environment"
	case strings.HasSuffix(lower, ".local"):
		name = name[:len(name)-len(".local")] + "-local"
	}
	return name + ".env"
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
	for _, v := range variables {
		refs := model.FindVariables("${" + v.Name + "}")
		if len(refs) != 1 || refs[0].Name != v.Name {
			return nil, fmt.Errorf("cannot save environment variable named %q", v.Name)
		}
		out.WriteString(v.Name + "=" + dotenvValue(v.Value) + "\n")
	}
	return []byte(out.String()), nil
}

var (
	singleQuoted = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	// Double quotes can't escape "${", so a literal one is written as an
	// expansion of the empty name, which never has a value, defaulting to "$".
	// Posting and python-dotenv both read ${:-$}{ as ${.
	doubleQuoted = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "${", "${:-$}{")
)

// dotenvValue encodes a template so that loading it gives what
// model.Substitute would. Without references the value is single-quoted,
// which dotenv reads literally. References need double quotes, where dotenv
// expands ${NAME}.
func dotenvValue(template string) string {
	refs := model.FindVariables(template)
	if len(refs) == 0 {
		return "'" + singleQuoted.Replace(strings.ReplaceAll(template, "$$", "$")) + "'"
	}
	var b strings.Builder
	b.WriteByte('"')
	last := 0
	for _, ref := range refs {
		b.WriteString(doubleQuoted.Replace(strings.ReplaceAll(template[last:ref.Start], "$$", "$")))
		b.WriteString("${" + ref.Name + "}")
		last = ref.End
	}
	b.WriteString(doubleQuoted.Replace(strings.ReplaceAll(template[last:], "$$", "$")))
	b.WriteByte('"')
	return b.String()
}
