// Package env loads environments: layered dotenv files whose values fill in
// ${VARIABLE} references in requests.
package env

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

// Pair is one assignment from a dotenv file, in file order.
type Pair struct {
	Name  string
	Value string
}

// Parse reads dotenv content the way python-dotenv (and so Posting 2) does:
//
//	KEY=value            # comments after unquoted values are dropped
//	export KEY=value
//	KEY="double quoted"  # \n, \t, \" and \\ escapes; may span lines
//	KEY='single quoted'  # literal
//	KEY=${OTHER}/path    # expanded from earlier keys, then the host
//
// Lines that aren't assignments are ignored.
func Parse(data string, lookupHost func(string) (string, bool)) []Pair {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	var pairs []Pair
	values := map[string]string{}
	lookup := func(name string) (string, bool) {
		if v, ok := values[name]; ok {
			return v, true
		}
		if lookupHost != nil {
			return lookupHost(name)
		}
		return "", false
	}
	for len(data) > 0 {
		line := data
		rest := ""
		if i := strings.IndexByte(data, '\n'); i >= 0 {
			line, rest = data[:i], data[i+1:]
		}
		data = rest
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		name, raw, ok := strings.Cut(trimmed, "=")
		name = strings.TrimSpace(name)
		if !ok || !validName(name) {
			continue
		}
		raw = strings.TrimLeft(raw, " \t")
		var value string
		switch {
		case strings.HasPrefix(raw, `"`):
			// A double-quoted value may continue onto following lines.
			body := raw[1:]
			for {
				if end := closingQuote(body, '"'); end >= 0 {
					value = expand(unescape(body[:end]), lookup)
					break
				}
				if data == "" {
					value = expand(unescape(body), lookup)
					break
				}
				next := data
				data = ""
				if i := strings.IndexByte(next, '\n'); i >= 0 {
					next, data = next[:i], next[i+1:]
				}
				body += "\n" + next
			}
		case strings.HasPrefix(raw, `'`):
			body := raw[1:]
			for {
				if end := strings.IndexByte(body, '\''); end >= 0 {
					value = body[:end]
					break
				}
				if data == "" {
					value = body
					break
				}
				next := data
				data = ""
				if i := strings.IndexByte(next, '\n'); i >= 0 {
					next, data = next[:i], next[i+1:]
				}
				body += "\n" + next
			}
		default:
			if i := strings.Index(raw, " #"); i >= 0 {
				raw = raw[:i]
			}
			if i := strings.Index(raw, "\t#"); i >= 0 {
				raw = raw[:i]
			}
			value = expand(strings.TrimSpace(raw), lookup)
		}
		values[name] = value
		pairs = append(pairs, Pair{Name: name, Value: value})
	}
	return pairs
}

// closingQuote finds the unescaped quote ending a double-quoted value.
func closingQuote(s string, quote byte) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return -1
}

func unescape(s string) string {
	replacer := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r", `\"`, `"`, `\\`, `\`, `\'`, `'`)
	return replacer.Replace(s)
}

// expand replaces ${NAME} and ${NAME:-default}; unknown names become empty,
// as in python-dotenv. Bare $NAME is left alone.
func expand(s string, lookup func(string) (string, bool)) string {
	if !strings.Contains(s, "${") {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := strings.IndexByte(s[start:], '}')
		if end < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:start])
		expr := s[start+2 : start+end]
		name, fallback, hasDefault := strings.Cut(expr, ":-")
		if value, ok := lookup(name); ok && (value != "" || !hasDefault) {
			b.WriteString(value)
		} else if hasDefault {
			b.WriteString(fallback)
		}
		s = s[start+end+1:]
	}
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		switch {
		case c == '_', c == '.', c == '-', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// Load reads and layers the files: a variable in a later file overrides the
// same variable in an earlier one. The environment is named after the files.
func Load(files []string) (model.Environment, error) {
	environment := model.Environment{Name: Name(files), Files: files}
	index := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return model.Environment{}, err
		}
		source := filepath.Base(file)
		for _, pair := range Parse(string(data), os.LookupEnv) {
			v := model.Variable{Name: pair.Name, Value: pair.Value, Source: source}
			if i, ok := index[pair.Name]; ok {
				environment.Variables[i] = v
				continue
			}
			index[pair.Name] = len(environment.Variables)
			environment.Variables = append(environment.Variables, v)
		}
	}
	sort.Slice(environment.Variables, func(i, j int) bool { return environment.Variables[i].Name < environment.Variables[j].Name })
	return environment, nil
}

// Name labels a set of files the way the switcher shows them.
func Name(files []string) string {
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = filepath.Base(file)
	}
	return strings.Join(names, " + ")
}

// IsCandidate reports whether a file name looks like an environment file:
// .env, *.env or .env.*.
func IsCandidate(name string) bool {
	return name == ".env" || strings.HasSuffix(name, ".env") || strings.HasPrefix(name, ".env.")
}

// Discover lists the environment files directly inside dirs, as absolute
// paths, sorted and without duplicates.
func Discover(dirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !IsCandidate(entry.Name()) {
				continue
			}
			path, err := filepath.Abs(filepath.Join(dir, entry.Name()))
			if err != nil || seen[path] {
				continue
			}
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				continue
			}
			seen[path] = true
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// Host returns the process environment as variables, for the
// use_host_environment setting.
func Host() []model.Variable {
	var out []model.Variable
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name != "" {
			out = append(out, model.Variable{Name: name, Value: value, Source: "host"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Source finds environment files in a set of directories, as Posting 2's
// switcher does: the working directory, the collection and the config
// directory are searched, but not their subdirectories.
type Source struct {
	Dirs []string
}

// Candidates offers each environment file found on its own.
func (s Source) Candidates() [][]string {
	var out [][]string
	for _, file := range Discover(s.Dirs) {
		out = append(out, []string{file})
	}
	return out
}

// Load reads the files afresh.
func (s Source) Load(files []string) (model.Environment, error) { return Load(files) }
