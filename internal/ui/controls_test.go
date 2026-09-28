package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
)

func TestTabStripRevealScrollsOnlyAsFarAsNeeded(tt *testing.T) {
	// Each tab is 6 cells wide: " Tab0 ".
	var tabs []tabItem
	for _, key := range []string{"Tab0", "Tab1", "Tab2", "Tab3", "Tab4"} {
		tabs = append(tabs, tabItem{Key: key, Label: key})
	}
	view := newTabView()
	strip := tabStrip{ID: "strip", Tabs: tabs, Active: t.NewSignal("Tab0"), View: view}

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
	targets["g"].Action()
	if got := s.responseTab.Peek(); got != "trace" {
		tt.Errorf("after g: response tab = %q, want trace", got)
	}
	targets["4"].Action()
	if got := app.sidebarTab.Peek(); got != "history" {
		tt.Errorf("after 4: sidebar tab = %q, want history", got)
	}
}
