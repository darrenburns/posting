package ui

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// clickID renders app and clicks the top-left cell of the widget with id, as
// the app's mouse routing does: the click goes to the widget registered under
// the pointer, which is the widget as written, before Build.
func clickID(tt *testing.T, app *App, id string) {
	tt.Helper()
	renderer := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(app)
	target := renderer.WidgetByID(id)
	if target == nil || target.Visible.IsEmpty() {
		tt.Fatalf("%s is not on screen", id)
	}
	entry := renderer.WidgetAt(target.Visible.X, target.Visible.Y)
	clickable, ok := entry.EventWidget.(t.Clickable)
	if !ok {
		tt.Fatalf("clicking %s reaches %T (%q), which doesn't handle clicks", id, entry.EventWidget, entry.ID)
	}
	clickable.OnClick(t.MouseEvent{X: target.Visible.X, Y: target.Visible.Y, Button: uv.MouseLeft, ClickCount: 1})
}

func TestClickMethodSelectorOpensMenu(tt *testing.T) {
	app := testApp()
	clickID(tt, app, methodSelectorID)
	if !app.menuOpen.Peek() {
		tt.Fatal("the method menu didn't open")
	}
}

func TestClickTabs(tt *testing.T) {
	app := testApp()
	clickID(tt, app, tabID(sidebarTabsID, "history"))
	if got := app.sidebarTab.Peek(); got != "history" {
		tt.Errorf("sidebar tab = %q, want history", got)
	}
	clickID(tt, app, tabID(requestTabsID, "auth"))
	if got := app.current().requestTab.Peek(); got != "auth" {
		tt.Errorf("request tab = %q, want auth", got)
	}
}

// pressOn runs the first of the focusable widget id's keybinds that matches
// key, as the focus manager does when the widget has focus.
func pressOn(tt *testing.T, app *App, id, key string) {
	tt.Helper()
	renderer := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	for _, entry := range renderer.Render(app) {
		if entry.ID != id {
			continue
		}
		provider, ok := entry.Focusable.(t.KeybindProvider)
		if !ok {
			tt.Fatalf("%s (%T) has no keybinds", id, entry.Focusable)
		}
		for _, kb := range provider.Keybinds() {
			if kb.Key == key {
				kb.Action()
				return
			}
		}
		tt.Fatalf("%s has no keybind for %q", id, key)
	}
	tt.Fatalf("%s is not focusable on screen", id)
}

func TestMethodHotkeysInOpenMenu(tt *testing.T) {
	for _, method := range model.Methods {
		app := testApp()
		app.openMethodMenu()
		pressOn(tt, app, methodMenuID, methodHotkeys[method])
		if got := app.current().method.Peek(); got != method {
			tt.Errorf("pressing %q in the open menu: method = %s, want %s", methodHotkeys[method], got, method)
		}
		if app.menuOpen.Peek() {
			tt.Errorf("pressing %q in the open menu left it open", methodHotkeys[method])
		}
	}
}
