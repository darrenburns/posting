// Package config loads Posting's settings.
//
// The schema is Posting 2's, so an existing config.yaml keeps working.
// Settings come from, in increasing priority: the defaults, the config file
// ($POSTING_CONFIG_FILE, or config.yaml in the XDG config directory), and
// POSTING_* environment variables, where nested keys are joined with a
// double underscore (POSTING_HEADING__SHOW_HOST=false).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/darrenburns/posting/internal/paths"
)

// Settings is the whole configuration.
type Settings struct {
	// Theme is the name of the theme to use.
	Theme string `yaml:"theme"`
	// ThemeDirectory holds user themes (Posting 2's theme YAML format).
	ThemeDirectory    string `yaml:"theme_directory"`
	LoadUserThemes    bool   `yaml:"load_user_themes"`
	LoadBuiltinThemes bool   `yaml:"load_builtin_themes"`
	// Layout places the response beside ("horizontal") or below
	// ("vertical") the request.
	Layout string `yaml:"layout"`
	// UseHostEnvironment makes the process's environment variables
	// available to requests, beneath the environment files.
	UseHostEnvironment bool `yaml:"use_host_environment"`
	// WatchEnvFiles reloads the active environment when its files change.
	WatchEnvFiles bool `yaml:"watch_env_files"`
	// WatchCollectionFiles reloads the collection when its files change.
	WatchCollectionFiles bool `yaml:"watch_collection_files"`
	// WatchThemes reloads user themes when they change.
	WatchThemes bool `yaml:"watch_themes"`

	TextInput         TextInputSettings         `yaml:"text_input"`
	Animation         string                    `yaml:"animation"`
	History           HistorySettings           `yaml:"history"`
	Response          ResponseSettings          `yaml:"response"`
	Heading           HeadingSettings           `yaml:"heading"`
	URLBar            URLBarSettings            `yaml:"url_bar"`
	CollectionBrowser CollectionBrowserSettings `yaml:"collection_browser"`
	CommandPalette    CommandPaletteSettings    `yaml:"command_palette"`

	Pager     string `yaml:"pager"`
	PagerJSON string `yaml:"pager_json"`
	Editor    string `yaml:"editor"`

	UseXresources bool                `yaml:"use_xresources"`
	SSL           CertificateSettings `yaml:"ssl"`
	Focus         FocusSettings       `yaml:"focus"`
	// Keymap rebinds actions by ID, e.g. {"send-request": "ctrl+r"}.
	// Several keys may be given, separated by commas.
	Keymap map[string]string `yaml:"keymap"`
	// CurlExportExtraArgs are inserted after "curl" in exported commands.
	CurlExportExtraArgs string `yaml:"curl_export_extra_args"`
	// Spacing is "standard", or "compact" to drop blank separator rows.
	Spacing string `yaml:"spacing"`
	// NerdFonts uses Nerd Font icons. Unset means on in terminals known to
	// ship them (Ghostty), off elsewhere.
	NerdFonts *bool `yaml:"nerd_fonts"`
}

type TextInputSettings struct {
	BlinkingCursor bool `yaml:"blinking_cursor"`
}

type HistorySettings struct {
	// Enabled keeps responses on disk for the History tab.
	Enabled bool `yaml:"enabled"`
}

type ResponseSettings struct {
	PrettifyJSON    bool `yaml:"prettify_json"`
	ShowSizeAndTime bool `yaml:"show_size_and_time"`
}

type HeadingSettings struct {
	Visible     bool   `yaml:"visible"`
	ShowHost    bool   `yaml:"show_host"`
	ShowVersion bool   `yaml:"show_version"`
	Hostname    string `yaml:"hostname"`
}

type URLBarSettings struct {
	ShowValuePreview          bool `yaml:"show_value_preview"`
	HideSecretsInValuePreview bool `yaml:"hide_secrets_in_value_preview"`
}

type CollectionBrowserSettings struct {
	Position      string `yaml:"position"`
	ShowOnStartup bool   `yaml:"show_on_startup"`
}

type CommandPaletteSettings struct {
	ThemePreview bool `yaml:"theme_preview"`
}

type CertificateSettings struct {
	CABundle        string `yaml:"ca_bundle"`
	CertificatePath string `yaml:"certificate_path"`
	KeyFile         string `yaml:"key_file"`
	Password        string `yaml:"password"`
}

type FocusSettings struct {
	OnStartup     string `yaml:"on_startup"`
	OnResponse    string `yaml:"on_response"`
	OnRequestOpen string `yaml:"on_request_open"`
}

// Defaults are Posting 2's defaults, except the theme preview, which is on:
// seeing a theme before choosing it is the point of browsing them.
func Defaults() Settings {
	return Settings{
		Theme:                "galaxy",
		ThemeDirectory:       paths.ThemeDir(),
		LoadUserThemes:       true,
		LoadBuiltinThemes:    true,
		Layout:               "vertical",
		WatchEnvFiles:        true,
		WatchCollectionFiles: true,
		WatchThemes:          true,
		TextInput:            TextInputSettings{BlinkingCursor: true},
		Animation:            "none",
		History:              HistorySettings{Enabled: true},
		Response:             ResponseSettings{PrettifyJSON: true, ShowSizeAndTime: true},
		Heading:              HeadingSettings{Visible: true, ShowHost: true, ShowVersion: true},
		URLBar:               URLBarSettings{ShowValuePreview: true, HideSecretsInValuePreview: true},
		CollectionBrowser:    CollectionBrowserSettings{Position: "left", ShowOnStartup: true},
		CommandPalette:       CommandPaletteSettings{ThemePreview: true},
		Pager:                os.Getenv("PAGER"),
		Editor:               os.Getenv("EDITOR"),
		Keymap:               map[string]string{},
		Spacing:              "standard",
	}
}

// Load reads the config file and environment. Problems are returned as
// warnings: a bad setting falls back to its default rather than stopping
// the app.
func Load() (Settings, []string) {
	return load(paths.ConfigFile(), os.Environ())
}

func load(file string, environ []string) (Settings, []string) {
	s := Defaults()
	var warnings []string
	if data, err := os.ReadFile(file); err == nil {
		if err := yaml.Unmarshal(data, &s); err != nil {
			warnings = append(warnings, fmt.Sprintf("Couldn't read %s: %v", file, err))
			s = Defaults()
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, fmt.Sprintf("Couldn't read %s: %v", file, err))
	}
	if node := environmentNode(environ); node != nil {
		if err := node.Decode(&s); err != nil {
			warnings = append(warnings, "Ignoring POSTING_ environment settings: "+err.Error())
		}
	}
	if s.Keymap == nil {
		s.Keymap = map[string]string{}
	}
	return s, append(warnings, s.validate()...)
}

// environmentNode turns POSTING_* variables into a YAML mapping, so their
// values are parsed exactly as they would be in the file.
func environmentNode(environ []string) *yaml.Node {
	type entry struct {
		path  []string
		value string
	}
	var entries []entry
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(strings.ToUpper(name), "POSTING_") || value == "" {
			continue
		}
		key := strings.ToLower(name[len("POSTING_"):])
		if key == "config_file" || key == "" {
			continue
		}
		entries = append(entries, entry{path: strings.Split(key, "__"), value: value})
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return strings.Join(entries[i].path, ".") < strings.Join(entries[j].path, ".") })
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, e := range entries {
		node := root
		for i, part := range e.path {
			var child *yaml.Node
			for j := 0; j+1 < len(node.Content); j += 2 {
				if node.Content[j].Value == part {
					child = node.Content[j+1]
				}
			}
			if child == nil {
				key := &yaml.Node{Kind: yaml.ScalarNode, Value: part}
				child = &yaml.Node{Kind: yaml.MappingNode}
				if i == len(e.path)-1 {
					child = &yaml.Node{Kind: yaml.ScalarNode, Value: envScalar(e.path, e.value)}
				}
				node.Content = append(node.Content, key, child)
			}
			node = child
		}
	}
	return root
}

// envScalar accepts the spellings of booleans pydantic accepts in
// environment variables, which YAML alone doesn't read as booleans. String
// settings must retain their exact values, even when they look like booleans.
func envScalar(path []string, value string) string {
	setting := reflect.TypeOf(Settings{})
	for _, part := range path {
		if setting.Kind() != reflect.Struct {
			return value
		}
		var fieldType reflect.Type
		for i := 0; i < setting.NumField(); i++ {
			field := setting.Field(i)
			if strings.Split(field.Tag.Get("yaml"), ",")[0] == part {
				fieldType = field.Type
				break
			}
		}
		if fieldType == nil {
			return value
		}
		setting = fieldType
	}
	if setting.Kind() == reflect.Pointer {
		setting = setting.Elem()
	}
	if setting.Kind() != reflect.Bool {
		return value
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "yes", "on", "true":
		return "true"
	case "0", "no", "off", "false":
		return "false"
	}
	return value
}

// validate resets settings with unknown values to their defaults.
func (s *Settings) validate() []string {
	d := Defaults()
	var warnings []string
	check := func(name string, value *string, fallback string, allowed ...string) {
		for _, a := range allowed {
			if *value == a {
				return
			}
		}
		warnings = append(warnings, fmt.Sprintf("Unknown %s %q; using %q", name, *value, fallback))
		*value = fallback
	}
	check("layout", &s.Layout, d.Layout, "vertical", "horizontal")
	check("spacing", &s.Spacing, d.Spacing, "standard", "compact")
	check("collection_browser.position", &s.CollectionBrowser.Position, d.CollectionBrowser.Position, "left", "right")
	check("focus.on_startup", &s.Focus.OnStartup, "url", "", "url", "method", "collection")
	check("focus.on_response", &s.Focus.OnResponse, "", "", "body", "tabs")
	check("focus.on_request_open", &s.Focus.OnRequestOpen, "", "", "headers", "body", "query", "info", "url", "method", "path")
	return warnings
}

// UseNerdFonts reports whether to draw Nerd Font icons: the nerd_fonts
// setting when it is set, otherwise whether the terminal is one that ships
// with a Nerd Font. Ghostty has Nerd Font symbols built in. lookup reads
// the environment (os.LookupEnv).
func (s Settings) UseNerdFonts(lookup func(string) (string, bool)) bool {
	if s.NerdFonts != nil {
		return *s.NerdFonts
	}
	get := func(name string) string { v, _ := lookup(name); return v }
	if strings.EqualFold(get("TERM_PROGRAM"), "ghostty") || get("TERM") == "xterm-ghostty" {
		return true
	}
	// Inside tmux or screen TERM_PROGRAM is the multiplexer, but Ghostty's
	// own variables are inherited.
	_, ghostty := lookup("GHOSTTY_RESOURCES_DIR")
	return ghostty
}

// KeysFor returns the keys bound to action, from the keymap or the
// defaults. Keys are comma-separated, as in Posting 2.
func (s Settings) KeysFor(action string, defaults ...string) []string {
	custom, ok := s.Keymap[action]
	if !ok || strings.TrimSpace(custom) == "" {
		return defaults
	}
	var keys []string
	for _, key := range strings.Split(custom, ",") {
		if key = strings.ToLower(strings.TrimSpace(key)); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}
