// Package themes reads user themes written in Posting 2's theme format.
package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Theme is a user theme. Colours are "#rrggbb" strings; empty ones keep the
// base theme's colour.
type Theme struct {
	Name       string `yaml:"name"`
	Primary    string `yaml:"primary"`
	Secondary  string `yaml:"secondary"`
	Accent     string `yaml:"accent"`
	Background string `yaml:"background"`
	Surface    string `yaml:"surface"`
	Panel      string `yaml:"panel"`
	Warning    string `yaml:"warning"`
	Error      string `yaml:"error"`
	Success    string `yaml:"success"`
	Text       string `yaml:"text"`
	Foreground string `yaml:"foreground"`
	// Dark themes build on a dark base theme, light ones on a light one.
	Dark *bool `yaml:"dark"`
	// File is where the theme was read from.
	File string `yaml:"-"`
}

// IsDark reports whether the theme is dark, which is the default.
func (t Theme) IsDark() bool { return t.Dark == nil || *t.Dark }

var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// Colours lists the theme's colour fields by name, for validation and use.
func (t Theme) Colours() map[string]string {
	return map[string]string{
		"primary": t.Primary, "secondary": t.Secondary, "accent": t.Accent,
		"background": t.Background, "surface": t.Surface, "panel": t.Panel,
		"warning": t.Warning, "error": t.Error, "success": t.Success,
		"text": t.Text, "foreground": t.Foreground,
	}
}

// Parse decodes and validates one theme file.
func Parse(data []byte) (Theme, error) {
	var theme Theme
	if err := yaml.Unmarshal(data, &theme); err != nil {
		return Theme{}, err
	}
	if strings.TrimSpace(theme.Name) == "" {
		return Theme{}, fmt.Errorf("the theme has no name")
	}
	if theme.Primary == "" {
		return Theme{}, fmt.Errorf("the theme has no primary colour")
	}
	for field, value := range theme.Colours() {
		if value != "" && !hexColour.MatchString(value) {
			return Theme{}, fmt.Errorf("%s colour %q isn't a hex colour like #1e88e5", field, value)
		}
	}
	return theme, nil
}

// LoadDir reads every *.yaml and *.yml theme in dir. A missing directory
// has no themes; files that fail to parse are reported and skipped.
func LoadDir(dir string) ([]Theme, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	var out []Theme
	var problems []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		theme, err := Parse(data)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		theme.File = path
		out = append(out, theme)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, problems
}
