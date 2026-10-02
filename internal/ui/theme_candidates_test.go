package ui

// Candidate dark themes, captured for review. THEME_CANDIDATES_OUT=<dir>
// renders a few scenes per candidate; docs/scripts/theme_gallery.py turns
// the captures into a gallery page.

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

type candidate struct {
	name, blurb                        string
	bg, text, muted, dim               string
	primary, secondary, accent         string
	errorColor, warning, success, info string
	cursor                             string // defaults to primary
}

var candidates = []candidate{
	{name: "fjord", blurb: "Slate water under a glacier: ice blue, lichen green, salmon.",
		bg: "#0e1418", text: "#dce6ea", muted: "#8a9ba4", dim: "#56656d",
		primary: "#86c5e4", secondary: "#a3c49b", accent: "#ee9e7d",
		errorColor: "#e8747c", warning: "#e8c77d", success: "#93d3a2", info: "#9fb4ea"},
	{name: "blueprint", blurb: "Cobalt drafting paper, cyan linework, pencil-yellow notes.",
		bg: "#0b1d3a", text: "#e2ecff", muted: "#8ea6cc", dim: "#58719a",
		primary: "#6fd3ff", secondary: "#ffd166", accent: "#ff8fa3",
		errorColor: "#ff6b7d", warning: "#ffb86b", success: "#7ee2a8", info: "#a9c4ff"},
	{name: "tidepool", blurb: "Rock pools at dusk: sea-glass teal, wet sand, anemone violet.",
		bg: "#0b1615", text: "#dcebe6", muted: "#87a39d", dim: "#536b66",
		primary: "#5ed3b2", secondary: "#e8b48a", accent: "#c99cf0",
		errorColor: "#f0818a", warning: "#f2cf7e", success: "#8fdc9a", info: "#7cc4e8"},
	{name: "sumi", blurb: "Ink on washi: warm greys and a single vermilion seal.",
		bg: "#141312", text: "#e8e4dc", muted: "#9b968c", dim: "#625e57",
		primary: "#e8553d", secondary: "#a9b8a0", accent: "#d9b26f",
		errorColor: "#f07a6a", warning: "#d9b26f", success: "#a9c49a", info: "#9fb2c4"},
	{name: "matcha", blurb: "Whisked green tea on dark olive lacquer, with a blush of peach.",
		bg: "#11140d", text: "#e6ead8", muted: "#9aa284", dim: "#626a50",
		primary: "#a8cf5a", secondary: "#e6c36a", accent: "#ec8f7e",
		errorColor: "#ec7474", warning: "#f0b45a", success: "#7fd18f", info: "#8fc2c9"},
	{name: "espresso", blurb: "Dark roast brown, crema gold and a sprig of mint.",
		bg: "#17110e", text: "#efe3d6", muted: "#a8968a", dim: "#6e5f55",
		primary: "#d9a066", secondary: "#8cc4b0", accent: "#e9715a",
		errorColor: "#e66a5e", warning: "#f0c36a", success: "#9ccf8a", info: "#8cb4d9"},
	{name: "outrun", blurb: "Sunset grid: hot magenta, laser cyan, chrome orange.",
		bg: "#160a28", text: "#f6e8ff", muted: "#a68cc4", dim: "#6c568a",
		primary: "#ff3fa4", secondary: "#36e8d8", accent: "#ffae42",
		errorColor: "#ff4f6d", warning: "#ffd23f", success: "#4dffb0", info: "#7aa8ff"},
	{name: "lacquer", blurb: "Cinnabar lacquerware with gold leaf and jade inlay.",
		bg: "#150c0b", text: "#f2e4dc", muted: "#ad938a", dim: "#735b54",
		primary: "#e5493a", secondary: "#d8b56a", accent: "#4fb3a5",
		errorColor: "#f2707a", warning: "#e8c06a", success: "#7fc99a", info: "#79b4d0"},
	{name: "verdigris", blurb: "Weathered copper: patina green over burnished metal.",
		bg: "#0d1513", text: "#dfece6", muted: "#8aa49b", dim: "#55706a",
		primary: "#4fbfa0", secondary: "#d98e5c", accent: "#e8c27a",
		errorColor: "#e8706a", warning: "#f0a85a", success: "#8fd49a", info: "#7fb8d4"},
	{name: "dusk", blurb: "Lavender twilight, rose clouds and the first star.",
		bg: "#15131e", text: "#ebe6f5", muted: "#9d95b3", dim: "#655e7a",
		primary: "#b7a1ee", secondary: "#f2a9c9", accent: "#f5d58a",
		errorColor: "#f08a9a", warning: "#f5c27a", success: "#9edcb4", info: "#8fb8f0"},
	{name: "yozakura", blurb: "Night cherry blossom beneath paper lanterns.",
		bg: "#120f16", text: "#f3e9ee", muted: "#a897a3", dim: "#6d5f6a",
		primary: "#f4a3c0", secondary: "#8fb0e8", accent: "#f2c879",
		errorColor: "#ff7a8a", warning: "#f2c879", success: "#a3d9a5", info: "#8fb0e8"},
	{name: "saffron", blurb: "Saffron and kumkum red dyed into indigo cloth.",
		bg: "#0f1120", text: "#ede9e0", muted: "#9b9cb5", dim: "#5f6280",
		primary: "#f5a623", secondary: "#4fc4b0", accent: "#e85d75",
		errorColor: "#ff5c6c", warning: "#f5d04a", success: "#7fd99a", info: "#7ea6f5"},
	{name: "graphite", blurb: "Neutral pro greys with a precise electric-blue edge.",
		bg: "#121314", text: "#e6e7e9", muted: "#9a9ca1", dim: "#616368",
		primary: "#4c9aff", secondary: "#b392f0", accent: "#ffab40",
		errorColor: "#ff5c5c", warning: "#ffb340", success: "#3ecf8e", info: "#4c9aff"},
	{name: "void", blurb: "True black for OLED screens, with acid-lime highlights.",
		bg: "#000000", text: "#f2f2f2", muted: "#8c8c8c", dim: "#555555",
		primary: "#c6f432", secondary: "#8b7bff", accent: "#ff5fa0",
		errorColor: "#ff4d5e", warning: "#ffc53d", success: "#4ade80", info: "#58b8ff"},
	{name: "noir", blurb: "Black-and-white film stock with one red accent.",
		bg: "#0f0f0f", text: "#ededed", muted: "#9a9a9a", dim: "#5e5e5e",
		primary: "#e6e6e6", secondary: "#b0b0b0", accent: "#ff3b30",
		errorColor: "#ff3b30", warning: "#d8c690", success: "#a8c5a0", info: "#a0b4c8"},
	{name: "fig", blurb: "Ripe fig flesh, sage leaves and a drizzle of honey.",
		bg: "#191116", text: "#f2e6ec", muted: "#ab95a2", dim: "#715d69",
		primary: "#d184a8", secondary: "#a7bf8f", accent: "#ecbd6c",
		errorColor: "#f07a7a", warning: "#ecbd6c", success: "#9fd49a", info: "#92b3e0"},
	{name: "mojito", blurb: "Crushed mint, lime and a muddled strawberry.",
		bg: "#0c1512", text: "#e4f5ee", muted: "#8aaa9d", dim: "#557367",
		primary: "#5ef0b0", secondary: "#c8f06a", accent: "#ff7f9e",
		errorColor: "#ff6b81", warning: "#ffd166", success: "#8ef08a", info: "#6cc8ff"},
	{name: "tangerine", blurb: "Bright citrus on warm charcoal, cooled by a teal twist.",
		bg: "#141210", text: "#f4ece4", muted: "#a99e93", dim: "#6d645b",
		primary: "#ff8a3d", secondary: "#5cc8c4", accent: "#ffd23f",
		errorColor: "#ff5a5a", warning: "#ffd23f", success: "#8fd46a", info: "#5cc8c4"},
	{name: "cosmos", blurb: "Deep space: star gold, nebula blue and pulsar pink.",
		bg: "#07080f", text: "#e8eaf6", muted: "#8d91ad", dim: "#545873",
		primary: "#f5c065", secondary: "#6f8cff", accent: "#ec6fa8",
		errorColor: "#ff6b7a", warning: "#ffa95e", success: "#6fe0a8", info: "#6f8cff"},
	{name: "peacock", blurb: "Iridescent cyan and sapphire feathers with a gold eye.",
		bg: "#061219", text: "#ddeef2", muted: "#84a2ad", dim: "#4c6a75",
		primary: "#12b5c4", secondary: "#6a7cff", accent: "#e8b84a",
		errorColor: "#f06a7a", warning: "#e8b84a", success: "#5fdc9a", info: "#6a7cff"},
	{name: "oxide", blurb: "Industrial rust, blued steel and safety yellow.",
		bg: "#121416", text: "#e4e6e8", muted: "#959ba1", dim: "#5d6369",
		primary: "#d9653b", secondary: "#8ca3b8", accent: "#f0c53a",
		errorColor: "#ef5a4a", warning: "#f0c53a", success: "#9cc479", info: "#8ca3b8"},
	{name: "bubblegum", blurb: "Candy pastels: bubblegum pink, sky blue and lime soda.",
		bg: "#19141f", text: "#fbeefa", muted: "#b39dbb", dim: "#77637f",
		primary: "#ff8ad8", secondary: "#7fdcff", accent: "#c8f58a",
		errorColor: "#ff6f91", warning: "#ffd98a", success: "#9cf2b0", info: "#7fdcff"},
	{name: "thunderhead", blurb: "Storm-cloud slate lit by a flash of lightning.",
		bg: "#12161b", text: "#e1e7ee", muted: "#8d99a8", dim: "#566170",
		primary: "#f4c84a", secondary: "#7fa6d1", accent: "#d47d8a",
		errorColor: "#e86a72", warning: "#f4a64a", success: "#7ccf9a", info: "#7fa6d1"},
	{name: "ultraviolet", blurb: "Blacklight club: UV violet and fluorescent green.",
		bg: "#0d0717", text: "#efe6ff", muted: "#9e8cc0", dim: "#635386",
		primary: "#b06bff", secondary: "#3dffb0", accent: "#ff5c8a",
		errorColor: "#ff4d6d", warning: "#ffd23d", success: "#3dffb0", info: "#6bb6ff"},
	{name: "darkroom", blurb: "A photo darkroom glowing under the red safelight.",
		bg: "#120909", text: "#f5dcd6", muted: "#b08a82", dim: "#75564f",
		primary: "#ff5544", secondary: "#e0a87a", accent: "#f2d27a",
		errorColor: "#ff7a6a", warning: "#f2d27a", success: "#c4c48a", info: "#d9a3a3"},
	{name: "nightshade", blurb: "Belladonna: deep blue-violet, berry magenta and amber.",
		bg: "#0f0e1c", text: "#e6e6f7", muted: "#9392b8", dim: "#5b5a80",
		primary: "#7f9cf5", secondary: "#e27cf0", accent: "#fbbf3a",
		errorColor: "#f2668b", warning: "#fbbf3a", success: "#6ee0b0", info: "#7f9cf5"},
	{name: "lichen", blurb: "Soft grey-green stone with pastel lichen blooms.",
		bg: "#131613", text: "#e3e8e1", muted: "#969f94", dim: "#5f685d",
		primary: "#b8cc72", secondary: "#9db8c8", accent: "#e3a3c0",
		errorColor: "#e48a8a", warning: "#e3c47a", success: "#a3d39a", info: "#9db8c8"},
	{name: "beacon", blurb: "Maximum legibility: pure black, pure white, saturated signals.",
		bg: "#000000", text: "#ffffff", muted: "#c4c4c4", dim: "#8a8a8a",
		primary: "#ffd400", secondary: "#00d8ff", accent: "#ff6bff",
		errorColor: "#ff5252", warning: "#ffa400", success: "#3dff7a", info: "#00d8ff"},
	{name: "evergreen", blurb: "Pine forest at night around a terracotta campfire.",
		bg: "#0b1410", text: "#e0ece4", muted: "#89a394", dim: "#536c5e",
		primary: "#6fcf8f", secondary: "#d6b583", accent: "#e3775c",
		errorColor: "#e86a5f", warning: "#e8c26a", success: "#9be07a", info: "#7fb8cf"},
	{name: "opal", blurb: "Iridescent opal: mint, lilac and peach on smoky quartz.",
		bg: "#121219", text: "#eeeef5", muted: "#9b9bb0", dim: "#626275",
		primary: "#92e3d6", secondary: "#c6a8f2", accent: "#f7b4a3",
		errorColor: "#f78a9a", warning: "#f5d38a", success: "#9fe3a8", info: "#9bc4f5"},
}

// contrast is the WCAG contrast ratio between two colours.
func contrast(a, b t.Color) float64 {
	la, lb := a.Luminance(), b.Luminance()
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func (c candidate) theme() t.ThemeData {
	bg, text := t.Hex(c.bg), t.Hex(c.text)
	primary := t.Hex(c.primary)
	// Surfaces rise from the background toward a primary-tinted light, so
	// panels and inputs stand clear of the background in the theme's own hue.
	light := text.Blend(primary, 0.35)
	surface := bg.Blend(light, 0.09)
	lift := func(r float64) t.Color { return surface.Blend(light, r) }
	// Text laid on a coloured fill: whichever of bg and text reads better.
	on := func(fill t.Color) t.Color {
		if contrast(fill, bg) >= contrast(fill, text) {
			return bg
		}
		return text
	}
	cursor := primary
	if c.cursor != "" {
		cursor = t.Hex(c.cursor)
	}
	data := t.ThemeData{
		Primary:   primary,
		Secondary: t.Hex(c.secondary),
		Accent:    t.Hex(c.accent),

		Background:   bg,
		Surface:      surface,
		SurfaceHover: lift(0.07),
		Surface2:     lift(0.11),
		Surface3:     lift(0.17),

		Text:         text,
		TextMuted:    t.Hex(c.muted),
		TextDisabled: t.Hex(c.dim),

		Border:    lift(0.17),
		FocusRing: primary,

		Error:   t.Hex(c.errorColor),
		Warning: t.Hex(c.warning),
		Success: t.Hex(c.success),
		Info:    t.Hex(c.info),

		ActiveCursor:  cursor,
		Selection:     cursor.WithAlpha(t.DefaultSelectionAlpha),
		SelectionText: on(cursor),

		ScrollbarTrack: surface,
		ScrollbarThumb: lift(0.22),

		Overlay: bg.WithAlpha(0.8),

		Placeholder: t.Hex(c.dim),
		Cursor:      primary,
		Link:        primary,
	}
	data.TextOnPrimary = on(data.Primary)
	data.TextOnSecondary = on(data.Secondary)
	data.TextOnAccent = on(data.Accent)
	data.TextOnError = on(data.Error)
	data.TextOnWarning = on(data.Warning)
	data.TextOnSuccess = on(data.Success)
	data.TextOnInfo = on(data.Info)
	return data
}

func TestCandidateThemeNamesAreNew(tt *testing.T) {
	seen := map[string]bool{}
	for _, name := range t.ThemeNames() {
		seen[name] = true
	}
	for _, c := range candidates {
		if seen[c.name] {
			tt.Errorf("%s is already a theme", c.name)
		}
		seen[c.name] = true
	}
	if len(candidates) != 30 {
		tt.Errorf("got %d candidates, want 30", len(candidates))
	}
}

func TestGenerateThemeCandidates(tt *testing.T) {
	out := os.Getenv("THEME_CANDIDATES_OUT")
	if out == "" {
		tt.Skip("THEME_CANDIDATES_OUT not set")
	}
	withResponse := func(th string) *App {
		app := homeApp(th)
		app.openRequest(sampleRequest(tt, "List users"))
		s := app.current()
		s.showResponse(fixedResponse(), nil)
		s.phase.Set(exchangeDone)
		return app
	}
	scenes := map[string]func(string) *App{
		"response": withResponse,
		"palette": func(th string) *App {
			app := withResponse(th)
			app.openPalette()
			return app
		},
		"history": func(th string) *App {
			app := homeApp(th)
			app.layout.Set(layoutHorizontal)
			app.sidebarTab.Set("history")
			sent := time.Date(2026, 9, 28, 14, 30, 0, 0, time.UTC)
			var history []model.HistoryEntry
			for i, name := range []string{"List users", "Create user", "Get user"} {
				req := sampleRequest(tt, name)
				resp := fixedResponse()
				resp.Method = req.Method
				if i == 2 {
					resp.StatusCode, resp.Reason = 404, "Not Found"
				}
				history = append(history, sentEntry(int64(i+1), req, resp, sent.Add(-time.Duration(i)*time.Minute)))
			}
			app.history.Set(history)
			app.historyList.SetItems(history)
			app.openHistory(history[0])
			app.current().responseTab.Set("trace")
			return app
		},
	}

	type entry struct {
		Name      string         `json:"name"`
		Blurb     string         `json:"blurb"`
		Reference bool           `json:"reference,omitempty"`
		Colors    map[string]any `json:"colors"`
	}
	for _, c := range candidates {
		t.RegisterTheme(c.name, c.theme())
	}
	// Built-in themes named in THEME_CANDIDATES_REFS are captured too, for
	// comparison.
	all := slices.Clone(candidates)
	for _, name := range strings.Split(os.Getenv("THEME_CANDIDATES_REFS"), ",") {
		if name != "" {
			all = append(all, candidate{name: name, blurb: "Built-in, for comparison."})
		}
	}
	var themes []entry
	result := map[string]map[string]t.SerializedBuffer{}
	for _, c := range all {
		th, _ := t.GetTheme(c.name)
		colors := map[string]any{}
		v := reflect.ValueOf(th)
		for i := 0; i < v.NumField(); i++ {
			if col, ok := v.Field(i).Interface().(t.Color); ok {
				colors[v.Type().Field(i).Name] = col.Hex()
			} else {
				colors[v.Type().Field(i).Name] = v.Field(i).Interface()
			}
		}
		themes = append(themes, entry{c.name, c.blurb, c.bg == "", colors})
	}
	for name, build := range scenes {
		result[name] = map[string]t.SerializedBuffer{}
		for _, c := range all {
			buf := t.RenderToBuffer(build(c.name), snapW, snapH)
			result[name][c.name] = t.SerializeBuffer(buf, snapW, snapH)
		}
	}
	for file, value := range map[string]any{"scenes.json": result, "themes.json": themes} {
		data, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(out, file), data, 0o644); err != nil {
			tt.Fatal(err)
		}
	}
}
