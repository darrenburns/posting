package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	uv "github.com/darrenburns/ultraviolet"

	"github.com/darrenburns/posting/v3/internal/config"
)

func TestNestedTabStyles(tt *testing.T) {
	for _, tc := range []struct {
		strip, active, inactive, blur string
		next                          [2]string
		show                          func(*App)
	}{
		{"req-body-type", "Raw", "Form", "req-body-text", [2]string{"Form", "Multipart"}, func(a *App) { a.current().requestTab.Set("body") }},
		{"req-body-content-type", "JSON", "XML", "req-body-text", [2]string{"Text", "XML"}, func(a *App) { a.current().requestTab.Set("body") }},
		{"req-auth-type", "Bearer token", "Basic", tabID(requestTabsID, "auth"), [2]string{"None", "Basic"}, func(a *App) { a.current().requestTab.Set("auth") }},
		{"curl-resolve", "Values", "Variables", "curl-text", [2]string{"Variables", "Values"}, func(a *App) { a.copyExport() }},
	} {
		tt.Run(tc.strip, func(tt *testing.T) {
			// Aurora's accent differs from its cursor, unlike Galaxy's.
			app := homeApp("aurora")
			app.openRequest(sampleRequest(tt, "Create user"))
			tc.show(app)
			p := t.NewPilot(tt, app, snapW, snapH)
			theme, _ := t.GetTheme("aurora")
			p.Click(tc.blur)
			bounds, ok := p.Bounds(tc.strip)
			if !ok {
				tt.Fatalf("%s is not visible", tc.strip)
			}
			x, y := findText(tt, p.ScreenText(), tc.active, bounds.X, bounds.X+bounds.Width)
			style := p.Buffer().CellAt(x, y).Style
			if style.Attrs&uv.AttrBold == 0 || style.Underline != uv.UnderlineSingle || t.FromANSI(style.UnderlineColor) != theme.ActiveCursor {
				tt.Fatal("selected, unfocused choice should be bold with a cursor-coloured underline")
			}
			p.ClickAt(x, y)
			style = p.Buffer().CellAt(x, y).Style
			if p.FocusedID() != tc.strip || t.FromANSI(style.Bg) != theme.ActiveCursor || t.FromANSI(style.Fg) != theme.SelectionText || style.Underline != uv.UnderlineNone {
				tt.Fatalf("focused choice should use the cursor fill and selection text; focus=%s style=%+v", p.FocusedID(), style)
			}
			// Repeated keys must use the latest selected value after each rebuild.
			for _, next := range tc.next {
				p.Press("right")
				x, y := findText(tt, p.ScreenText(), next, bounds.X, bounds.X+bounds.Width)
				if t.FromANSI(p.Buffer().CellAt(x, y).Style.Bg) != theme.ActiveCursor {
					tt.Fatalf("right should select %s", next)
				}
			}
			p.Press("left", "left")
			if t.FromANSI(p.Buffer().CellAt(x, y).Style.Bg) != theme.ActiveCursor {
				tt.Fatal("keyboard navigation did not return to the original choice")
			}
			p.Click(tc.blur)
			x, y = findText(tt, p.ScreenText(), tc.inactive, bounds.X, bounds.X+bounds.Width)
			before := *p.Buffer().CellAt(x, y)
			p.MouseMove(x, y)
			if got := t.FromANSI(p.Buffer().CellAt(x, y).Style.Bg); got != theme.Hover.BlendOver(t.FromANSI(before.Style.Bg)) {
				tt.Fatal("inactive choice did not gain the theme's hover tint")
			}
			p.MouseMove(0, 0)
			if !p.Buffer().CellAt(x, y).Equal(&before) {
				tt.Fatal("leaving the choice did not restore its appearance")
			}
			p.ClickAt(x, y)
			if t.FromANSI(p.Buffer().CellAt(x, y).Style.Bg) != theme.ActiveCursor {
				tt.Fatal("clicking the inactive choice did not select it")
			}
		})
	}
}

func TestPaneTabHover(tt *testing.T) {
	for _, name := range []string{"galaxy", "catppuccin-latte"} {
		tt.Run(name, func(tt *testing.T) {
			app := settingsApp(tt, func(s *config.Settings) {
				s.Theme = name
				s.SidebarGradient = "lantern"
			})
			app.openRequest(sampleRequest(tt, "Create user"))
			app.current().showResponse(fixedResponse(), nil)
			app.current().phase.Set(exchangeDone)
			p := t.NewPilot(tt, app, snapW, snapH)
			theme, _ := t.GetTheme(name)
			for _, tab := range []struct{ strip, active, inactive string }{
				{sidebarTabsID, "requests", "history"},
				{requestTabsID, "headers", "body"},  // Includes the content marker.
				{responseTabsID, "body", "headers"}, // Includes the count badge.
			} {
				p.MouseMove(0, snapH-1)
				id := tabID(tab.strip, tab.inactive)
				bounds, ok := p.Bounds(id)
				if !ok {
					tt.Fatalf("tab %s not on screen", id)
				}
				before := make([]uv.Cell, bounds.Width)
				for i := range before {
					before[i] = *p.Buffer().CellAt(bounds.X+i, bounds.Y)
				}
				// Hover includes both padding cells, not only the label.
				for _, x := range []int{bounds.X, bounds.X + bounds.Width - 1, 0} {
					p.MouseMove(x, bounds.Y)
					for i, want := range before {
						got := *p.Buffer().CellAt(bounds.X+i, bounds.Y)
						bg := t.FromANSI(want.Style.Bg)
						if x != 0 {
							bg = theme.Hover.BlendOver(bg)
						}
						if t.FromANSI(got.Style.Bg) != bg {
							tt.Fatalf("%s cell %d: background = %v, want %s (pointer x=%d)", id, i, got.Style.Bg, bg.Hex(), x)
						}
						got.Style.Bg = want.Style.Bg
						if !got.Equal(&want) {
							tt.Fatalf("%s cell %d: hover changed more than the background", id, i)
						}
					}
				}
				// Active styling wins over hover, with and without keyboard focus.
				activeID := tabID(tab.strip, tab.active)
				for _, focusID := range []string{activeID, urlInputID} {
					p.Click(focusID)
					p.MouseMove(0, snapH-1)
					activeBounds, _ := p.Bounds(activeID)
					before := *p.Buffer().CellAt(activeBounds.X+1, activeBounds.Y)
					p.Hover(activeID)
					if got := p.Buffer().CellAt(activeBounds.X+1, activeBounds.Y); !got.Equal(&before) {
						tt.Fatalf("hover changed the active tab %s (focus on %s)", activeID, focusID)
					}
				}
			}
		})
	}
}

func TestSelectedTabUnderlineMatchesCursor(tt *testing.T) {
	// Aurora's accent and cursor colours differ, so using Accent fails.
	app := settingsApp(tt, func(s *config.Settings) { s.Theme = "aurora" })
	app.current().showResponse(fixedResponse(), nil)
	app.current().phase.Set(exchangeDone)
	buf := uv.NewBuffer(snapW, snapH)
	focus := t.NewFocusManager()
	focused := t.NewAnySignal[t.Focusable](nil)
	renderer := t.NewRenderer(buf, snapW, snapH, focus, focused, t.NewAnySignal[t.Widget](nil))
	focus.SetFocusables(renderer.Render(app))
	for _, tab := range []struct{ strip, key string }{
		{sidebarTabsID, "requests"},
		{requestTabsID, "headers"},
		{responseTabsID, "body"},
	} {
		focus.FocusByID(urlInputID)
		focused.Set(focus.Focused())
		renderer.Render(app)
		bounds := renderer.WidgetByID(tabID(tab.strip, tab.key)).Bounds
		cell := buf.CellAt(bounds.X+1, bounds.Y)
		if cell.Style.Underline != uv.UnderlineSingle || cell.Style.UnderlineColor == nil {
			tt.Fatalf("%s: selected, unfocused tab has no coloured underline", tab.strip)
		}
		underline := t.FromANSI(cell.Style.UnderlineColor)
		focus.FocusByID(tab.strip)
		focused.Set(focus.Focused())
		renderer.Render(app)
		cell = buf.CellAt(bounds.X+1, bounds.Y)
		if cell.Style.Underline != uv.UnderlineNone || cell.Style.Bg == nil || underline != t.FromANSI(cell.Style.Bg) {
			tt.Fatalf("%s: underline %s does not match the focused cursor background %v", tab.strip, underline.Hex(), cell.Style.Bg)
		}
	}
}

func TestTabStripRevealScrollsOnlyAsFarAsNeeded(tt *testing.T) {
	// Each tab is 6 cells wide: " Tab0 ".
	var tabs []tabItem
	for _, key := range []string{"Tab0", "Tab1", "Tab2", "Tab3", "Tab4"} {
		tabs = append(tabs, tabItem{Key: key, Label: key})
	}
	view := newTabView()
	strip := tabStrip{ID: "strip", Tabs: tabs, Active: "Tab0", View: view}

	strip.selectKey("Tab4")
	if got := view.first.Peek(); got != 0 {
		tt.Fatalf("before the strip is painted: first = %d, want 0", got)
	}

	view.width = 20 // Room for three tabs, or two beside an overflow mark.
	for _, step := range []struct {
		key   string
		first int
	}{
		{"Tab1", 0}, // Already in view.
		{"Tab3", 2}, // Scrolls just far enough: ‹ Tab2 Tab3 ›
		{"Tab4", 2}, // In view once the › mark goes: ‹ Tab2 Tab3 Tab4
		{"Tab1", 1}, // Scrolls back to it.
		{"Tab0", 0},
	} {
		strip.selectKey(step.key)
		if got := view.first.Peek(); got != step.first {
			tt.Fatalf("select %s: first = %d, want %d", step.key, got, step.first)
		}
	}

	view.width = 40 // Wide enough for every tab.
	strip.View.first.Set(3)
	strip.selectKey("Tab4")
	if got := view.first.Peek(); got != 0 {
		tt.Fatalf("with room for every tab: first = %d, want 0", got)
	}
}

func TestJumpTargetsSelectTabs(tt *testing.T) {
	app := testApp()
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)

	targets := map[string]t.JumpTarget{}
	for _, target := range app.jumpTargets() {
		for key := range targets {
			if strings.HasPrefix(key, target.Key) || strings.HasPrefix(target.Key, key) {
				tt.Fatalf("jump keys %q and %q are ambiguous", key, target.Key)
			}
		}
		targets[target.Key] = target
	}

	targets["w"].Action()
	if got := s.requestTab.Peek(); got != "body" {
		tt.Errorf("after w: request tab = %q, want body", got)
	}
	targets["f"].Action()
	if got := s.responseTab.Peek(); got != "trace" {
		tt.Errorf("after f: response tab = %q, want trace", got)
	}
	targets["4"].Action()
	if got := app.sidebarTab.Peek(); got != "history" {
		tt.Errorf("after 4: sidebar tab = %q, want history", got)
	}
}
