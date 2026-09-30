// Package paths locates Posting's files. They follow the XDG base directory
// spec on every platform, as Posting 2 does, so both versions share a config
// file and default collection.
package paths

import (
	"os"
	"path/filepath"
)

// ConfigHome is $XDG_CONFIG_HOME, or ~/.config.
func ConfigHome() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataHome is $XDG_DATA_HOME, or ~/.local/share.
func DataHome() string { return xdg("XDG_DATA_HOME", filepath.Join(".local", "share")) }

// ConfigDir holds config.yaml.
func ConfigDir() string { return filepath.Join(ConfigHome(), "posting") }

// DataDir holds the default collection, themes and history.
func DataDir() string { return filepath.Join(DataHome(), "posting") }

// ConfigFile is the config file: $POSTING_CONFIG_FILE, or config.yaml in
// ConfigDir.
func ConfigFile() string {
	if file := os.Getenv("POSTING_CONFIG_FILE"); file != "" {
		if abs, err := filepath.Abs(file); err == nil {
			return abs
		}
		return file
	}
	return filepath.Join(ConfigDir(), "config.yaml")
}

// DefaultCollectionDir is used when no collection is given.
func DefaultCollectionDir() string { return filepath.Join(DataDir(), "default") }

// ThemeDir holds user themes.
func ThemeDir() string { return filepath.Join(DataDir(), "themes") }

// HistoryDir holds response history, one file per collection.
func HistoryDir() string { return filepath.Join(DataDir(), "history") }

// EnvironmentMemory remembers the environment last used in each collection.
func EnvironmentMemory() string { return filepath.Join(DataDir(), "environments.json") }

func xdg(variable, fallback string) string {
	if dir := os.Getenv(variable); dir != "" && filepath.IsAbs(dir) {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", fallback)
	}
	return filepath.Join(home, fallback)
}
