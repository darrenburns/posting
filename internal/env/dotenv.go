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
	var pairs []Pair
	scan(data, func(name, text string, expands bool) {
		if expands {
			text = expand(text, lookup)
		}
		values[name] = text
		pairs = append(pairs, Pair{Name: name, Value: text})
	})
	return pairs
}

// Templates reads dotenv content without expanding it, giving each value as
// a Posting template: ${NAME} stays a reference and a literal dollar sign
// becomes $$. A value that uses ${NAME:-default}, or refers to a name a
// template can't, has no template form; it is left out and named in skipped.
func Templates(data string) (pairs []Pair, skipped []string) {
	scan(data, func(name, text string, expands bool) {
		if !expands {
			pairs = append(pairs, Pair{Name: name, Value: strings.ReplaceAll(text, "$", "$$")})
			return
		}
		if value, ok := template(text); ok {
			pairs = append(pairs, Pair{Name: name, Value: value})
			return
		}
		skipped = append(skipped, name)
	})
	return pairs, skipped
}

// scan calls assign for each assignment in dotenv content with its decoded
// text, and whether ${...} in that text is to be expanded.
func scan(data string, assign func(name, text string, expands bool)) {
	for len(data) > 0 {
		line := data
		rest := ""
		if i := strings.IndexByte(data, '\n'); i >= 0 {
			line, rest = data[:i], data[i+1:]
		}
		data = rest
		trimmed := strings.TrimLeft(line, " \t\r")
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
		var text string
		expands := true
		switch {
		case strings.HasPrefix(raw, `"`):
			// A double-quoted value may continue onto following lines.
			body := raw[1:]
			for {
				if end := closingQuote(body, '"'); end >= 0 {
					text = unescape(body[:end])
					break
				}
				if data == "" {
					text = unescape(body)
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
			expands = false
			body := raw[1:]
			for {
				if end := closingQuote(body, '\''); end >= 0 {
					text = unescapeSingle(body[:end])
					break
				}
				if data == "" {
					text = unescapeSingle(body)
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
			text = strings.TrimSpace(raw)
		}
		assign(name, text, expands)
	}
}

// closingQuote finds the unescaped quote ending a quoted value.
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

// unescapeSingle follows dotenv's two escapes inside single quotes.
func unescapeSingle(s string) string {
	return strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(s)
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

// template is the Posting template for text that expand would read, or false
// if it has none. Nothing has the empty name, so ${:-x} is the literal x.
func template(s string) (string, bool) {
	var b strings.Builder
	literal := func(text string) { b.WriteString(strings.ReplaceAll(text, "$", "$$")) }
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			literal(s)
			return b.String(), true
		}
		end := strings.IndexByte(s[start:], '}')
		if end < 0 {
			literal(s)
			return b.String(), true
		}
		literal(s[:start])
		name, fallback, hasDefault := strings.Cut(s[start+2:start+end], ":-")
		switch refs := model.FindVariables("${" + name + "}"); {
		case name == "":
			if hasDefault {
				literal(fallback)
			}
		case hasDefault || len(refs) != 1 || refs[0].Name != name:
			return "", false
		default:
			b.WriteString("${" + name + "}")
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
// same variable in an earlier one, and a ${NAME} in a later file can use a
// value from an earlier one. The environment is named after the files.
func Load(files []string) (model.Environment, error) {
	environment := model.Environment{Name: Name(files), Files: files}
	loaded := map[string]string{}
	lookup := func(name string) (string, bool) {
		if v, ok := loaded[name]; ok {
			return v, true
		}
		return os.LookupEnv(name)
	}
	layers := make([][]model.Variable, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return model.Environment{}, err
		}
		source := filepath.Base(file)
		var layer []model.Variable
		for _, pair := range Parse(string(data), lookup) {
			layer = append(layer, model.Variable{Name: pair.Name, Value: pair.Value, Source: source})
		}
		for _, v := range layer {
			loaded[v.Name] = v.Value
		}
		layers = append(layers, layer)
	}
	environment.Variables = model.Merge(layers...)
	return environment, nil
}

// The layering convention. In a folder, posting.env is the base that every
// named environment builds on, and <name>.env is the environment itself.
// Either may have a .local.env companion for values kept out of version
// control, which layers directly above it:
//
//	posting.env → posting.local.env → staging.env → staging.local.env
const (
	BaseFile    = "posting.env"
	localSuffix = ".local.env"
	// BaseName is the name of the base environment on its own. It's named
	// after its file, as every environment is, so it can't collide with one:
	// base.env is an environment called "base", layered on posting.env.
	BaseName = "posting"
)

// localFile is the .local.env companion of an environment file name.
func localFile(name string) string { return strings.TrimSuffix(name, ".env") + localSuffix }

// isLocal reports whether a file name is a .local.env companion.
func isLocal(name string) bool { return strings.HasSuffix(name, localSuffix) && name != localSuffix }

// stem is the environment name a file name gives: staging.env and
// staging.local.env are both "staging". Names outside the convention (.env,
// .env.test) are kept whole.
func stem(name string) string {
	switch {
	case isLocal(name):
		return strings.TrimSuffix(name, localSuffix)
	case strings.HasSuffix(name, ".env") && name != ".env":
		return strings.TrimSuffix(name, ".env")
	}
	return name
}

// isBase reports whether a file name belongs to the base layers.
func isBase(name string) bool { return name == BaseFile || name == localFile(BaseFile) }

// Stack is the files of the environment called name in dir, in layering
// order, or nil if dir has no such environment. The base environment
// ("posting", or "") is posting.env and posting.local.env alone; any other name
// is layered on top of them. Only files that exist are included.
func Stack(dir, name string) []string {
	exists := func(file string) bool {
		info, err := os.Stat(filepath.Join(dir, file))
		return err == nil && !info.IsDir()
	}
	var own []string
	if name != "" && name != BaseName {
		file := name + ".env"
		if strings.ContainsRune(name, filepath.Separator) || isBase(file) {
			return nil
		}
		for _, f := range []string{file, localFile(file)} {
			if exists(f) {
				own = append(own, f)
			}
		}
		if len(own) == 0 {
			return nil
		}
	}
	var names []string
	for _, f := range []string{BaseFile, localFile(BaseFile)} {
		if exists(f) {
			names = append(names, f)
		}
	}
	names = append(names, own...)
	if len(names) == 0 {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	files := make([]string, len(names))
	for i, n := range names {
		files[i] = filepath.Join(abs, n)
	}
	return files
}

// Named finds the environment called name in the first of dirs that has it.
func Named(dirs []string, name string) []string {
	for _, dir := range dirs {
		if files := Stack(dir, name); files != nil {
			return files
		}
	}
	return nil
}

// Name labels a set of files the way the switcher shows them. A stack that
// follows the layering convention is named after its environment ("staging"
// for posting.env + staging.env + staging.local.env, "posting" for
// posting.env alone); anything else is named after its files.
func Name(files []string) string {
	names := make([]string, len(files))
	present := map[string]bool{}
	for i, file := range files {
		names[i] = filepath.Base(file)
		present[names[i]] = true
	}
	var own []string
	for _, n := range names {
		if isBase(n) || isLocal(n) && present[stem(n)+".env"] {
			continue
		}
		own = append(own, n)
	}
	switch {
	case len(files) == 0:
		return ""
	case len(own) == 0:
		return BaseName
	case len(own) == 1:
		return stem(own[0])
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

// Candidates offers the environments in each directory: the base on its
// own, then each named environment layered on it. Files outside the
// convention, such as .env, are offered on their own.
func (s Source) Candidates() [][]string {
	var out [][]string
	seen := map[string]bool{}
	for _, dir := range s.Dirs {
		abs, err := filepath.Abs(dir)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		files := Discover([]string{abs})
		present := map[string]bool{}
		for _, file := range files {
			present[filepath.Base(file)] = true
		}
		if base := Stack(abs, BaseName); base != nil {
			out = append(out, base)
		}
		for _, file := range files {
			name := filepath.Base(file)
			switch {
			case isBase(name), isLocal(name) && present[stem(name)+".env"]:
				// Part of the base, or of the environment it's local to.
			case isLocal(name), strings.HasSuffix(name, ".env") && name != ".env":
				if stack := Stack(abs, stem(name)); stack != nil {
					out = append(out, stack)
					break
				}
				// A name Stack won't resolve is still offered, on its own.
				out = append(out, []string{file})
			default:
				out = append(out, []string{file})
			}
		}
	}
	return out
}

// Load reads the files afresh.
func (s Source) Load(files []string) (model.Environment, error) { return Load(files) }
