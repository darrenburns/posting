package ui

import (
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/themes"
)

// registerUserThemes makes Posting 2 style user themes available to Terma.
// Each is built on a built-in theme (galaxy when dark, Catppuccin Latte when
// light) so the colours it doesn't set still fit together.
func registerUserThemes(list []themes.Theme) []string {
	names := make([]string, 0, len(list))
	for _, theme := range list {
		base := t.ThemeNameGalaxy
		if !theme.IsDark() {
			base = t.ThemeNameCatppuccinLatte
		}
		var opts []t.ThemeOption
		add := func(value string, option func(t.Color) t.ThemeOption) {
			if value != "" {
				opts = append(opts, option(t.Hex(value)))
			}
		}
		add(theme.Primary, t.WithPrimary)
		add(theme.Secondary, t.WithSecondary)
		add(theme.Accent, t.WithAccent)
		add(theme.Background, t.WithBackground)
		add(theme.Surface, t.WithSurface)
		add(theme.Panel, t.WithSurface2)
		add(theme.Warning, t.WithWarning)
		add(theme.Error, t.WithError)
		add(theme.Success, t.WithSuccess)
		add(theme.Foreground, t.WithText)
		add(theme.Text, t.WithText)
		data := t.ExtendTheme(base, opts...)
		data.Name = theme.Name
		data.IsLight = !theme.IsDark()
		t.RegisterTheme(theme.Name, data)
		names = append(names, theme.Name)
	}
	return names
}
