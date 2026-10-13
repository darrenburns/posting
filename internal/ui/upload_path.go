package ui

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
)

// uploadPath is shared by file parts and whole-file bodies. Completion only
// reads directory entries and metadata, never file contents.
type uploadPath struct {
	text        *t.TextInputState
	vars        *completion
	suggestions *t.AutocompleteState
	checked     string
	directory   string
	hint        t.Signal[string]
	problem     t.Signal[bool]
}

func newUploadPath(path string) *uploadPath {
	return &uploadPath{text: t.NewTextInputState(path), vars: newCompletion(), suggestions: t.NewAutocompleteState(),
		hint: t.NewSignal("Checking file…"), problem: t.NewSignal(false)}
}

func (a *App) uploadRoot() string {
	if dir, ok := a.store.(collection.Dir); ok {
		return dir.Root
	}
	return "."
}

func (a *App) resolvedUploadPath(s *Session, path string) string {
	if s.substitute.Checked.Get() {
		return model.Substitute(path, model.MapLookup(a.variableValues()))
	}
	return path
}

func (p *uploadPath) inspect(a *App, s *Session) {
	raw := p.text.GetText()
	resolved := a.resolvedUploadPath(s, raw)
	root := a.uploadRoot()
	key := root + "\x00" + raw + "\x00" + resolved
	if p.checked == key {
		return
	}
	p.checked = key
	prefix, _ := filepath.Split(raw)
	resolvedDir, _ := filepath.Split(resolved)
	directory := root + "\x00" + prefix + "\x00" + resolvedDir
	// Keep filtering the current entries while typing within a directory.
	// A different prefix or resolved directory makes those entries unsafe.
	if p.directory != directory {
		p.directory = directory
		p.suggestions.SetSuggestions(nil)
	}
	// Leave the previous hint and its color in place until the result is ready.
	go func() {
		hint, bad := "Choose a file", false
		if resolved != "" {
			info, err := os.Stat(model.UploadPath(root, resolved))
			switch {
			case os.IsNotExist(err):
				hint, bad = "File not found — check the path", true
			case err != nil:
				hint, bad = "File unavailable — check permissions", true
			case !info.Mode().IsRegular():
				hint, bad = "Choose a regular file, not a directory or device", true
			default:
				hint = model.FormatBytes(int(info.Size())) + " · " + model.FileContentType(resolved)
			}
		}
		var suggestions []t.Suggestion
		entries, _ := uploadDirectory(model.UploadPath(root, resolvedDir))
		for _, entry := range entries {
			value := prefix + entry.Name()
			if entry.IsDir() {
				value += string(filepath.Separator)
			}
			suggestions = append(suggestions, t.Suggestion{Label: value, Value: value})
		}
		a.dispatch(func() {
			if p.checked != key {
				return
			}
			p.hint.Set(hint)
			p.problem.Set(bad)
			p.suggestions.SetSuggestions(suggestions)
		})
	}()
}

// Bound each directory read; never recursively scan a collection or mount.
func uploadDirectory(dir string) ([]os.DirEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(512)
	if err != nil && err != io.EOF {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name(), b.Name())
	})
	return entries, nil
}

type uploadPathField struct {
	fillWidth
	path    *uploadPath
	app     *App
	session *Session
	id      string
	changed func()
}

func (v uploadPathField) Build(ctx t.BuildContext) t.Widget {
	p, a, s, theme := v.path, v.app, v.session, ctx.Theme()
	p.inspect(a, s)
	choices := variableChoices{}
	if s.substitute.Checked.Get() {
		choices = a.variableChoices()
	}
	// Terma needs a direct TextInput child, so select one completion source
	// instead of nesting the variable and file autocomplete widgets.
	suggestions, variables := p.suggestions, (*completion)(nil)
	before := strings.Join(p.text.Content.Get()[:p.text.CursorIndex.Get()], "")
	if dollar := strings.LastIndexByte(before, '$'); dollar >= 0 && len(choices.list) > 0 {
		name := strings.TrimPrefix(before[dollar+1:], "{")
		if !strings.ContainsFunc(name, func(r rune) bool { return r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			suggestions, variables = nil, p.vars
		}
	}
	popupWidth := 0
	for _, suggestion := range p.suggestions.Suggestions.Get() {
		popupWidth = max(popupWidth, ansi.StringWidth(suggestion.Label)+2)
	}
	color := theme.TextMuted
	if p.problem.Get() {
		color = theme.ErrorText
	}
	return t.Column{Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
		input{ID: v.id, State: p.text, Placeholder: "fixtures/report.pdf", Suggestions: suggestions, SuggestWidth: t.Cells(min(popupWidth, 48)), Completion: variables, Choices: choices,
			Highlighter: variableHighlighter(theme, a.resolver()), OnChange: func(string) { p.checked = ""; v.changed() },
			Keybinds: []t.Keybind{{Key: "f5", Name: "Refresh file", Action: func() { p.checked = ""; p.inspect(a, s) }}}},
		t.Text{Content: p.hint.Get(), Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: color, Padding: inset}},
	}}
}
