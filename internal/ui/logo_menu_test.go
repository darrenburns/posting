package ui

import (
	"fmt"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// logoApp is a test app whose links are sent to opened rather than opened.
func logoApp() (*App, chan string) {
	opened := make(chan string, 4)
	app := testApp()
	app.openURL = func(url string) error {
		opened <- url
		return nil
	}
	return app, opened
}

// nextOpened waits for the next link the app opens. Links open in the
// background.
func nextOpened(tt *testing.T, opened chan string) string {
	tt.Helper()
	select {
	case url := <-opened:
		return url
	case <-time.After(2 * time.Second):
		tt.Fatal("no link opened")
		return ""
	}
}

func TestClickLogoOpensMenu(tt *testing.T) {
	app := testApp()
	clickID(tt, app, logoID)
	if !app.logoMenuOpen.Peek() {
		tt.Fatal("clicking the logo didn't open its menu")
	}
}

func TestLogoMenuOpensLinks(tt *testing.T) {
	for i, want := range []string{docsURL, sponsorURL, mastodonURL} {
		app, opened := logoApp()
		clickID(tt, app, logoID)
		for range i {
			pressOn(tt, app, logoMenuID, "down")
		}
		pressOn(tt, app, logoMenuID, "enter")
		if got := nextOpened(tt, opened); got != want {
			tt.Errorf("choosing item %d opened %q, want %q", i, got, want)
		}
		if app.logoMenuOpen.Peek() {
			tt.Errorf("choosing item %d left the menu open", i)
		}
	}
}

func TestLogoMenuStartsAtTheTop(tt *testing.T) {
	app, opened := logoApp()
	clickID(tt, app, logoID)
	pressOn(tt, app, logoMenuID, "down")
	pressOn(tt, app, logoMenuID, "escape")
	clickID(tt, app, logoID)
	pressOn(tt, app, logoMenuID, "enter")
	if got := nextOpened(tt, opened); got != docsURL {
		tt.Errorf("reopening the menu and choosing opened %q, want the docs", got)
	}
}

func TestLogoMenuEscapeCloses(tt *testing.T) {
	app, opened := logoApp()
	clickID(tt, app, logoID)
	pressOn(tt, app, logoMenuID, "escape")
	if app.logoMenuOpen.Peek() {
		tt.Error("escape left the logo menu open")
	}
	if len(opened) > 0 {
		tt.Errorf("escape opened %q", <-opened)
	}
}

func TestLogoMenuRemembersFocus(tt *testing.T) {
	app := testApp()
	s := newScreen(app, snapW, snapH)
	s.focusID(tt, urlInputID)
	logo := s.renderer.WidgetByID(logoID)
	if logo == nil || logo.Visible.IsEmpty() {
		tt.Fatal("the logo is not on screen")
	}
	clickable, ok := s.renderer.WidgetAt(logo.Visible.X, logo.Visible.Y).EventWidget.(t.Clickable)
	if !ok {
		tt.Fatal("the logo doesn't handle clicks")
	}
	clickable.OnClick(t.MouseEvent{X: logo.Visible.X, Y: logo.Visible.Y, Button: uv.MouseLeft, ClickCount: 1})
	if !app.logoMenuOpen.Peek() {
		tt.Fatal("clicking the logo didn't open its menu")
	}
	if app.logoReturnFocus != urlInputID {
		tt.Errorf("the menu will hand focus back to %q, want %q", app.logoReturnFocus, urlInputID)
	}
}

func TestLogoBrightensUnderPointer(tt *testing.T) {
	app := testApp()
	color := func(hovered t.Widget) string {
		buf := uv.NewBuffer(snapW, snapH)
		renderer := t.NewRenderer(buf, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal(hovered))
		renderer.Render(app)
		logo := renderer.WidgetByID(logoID)
		if logo == nil || logo.Visible.IsEmpty() {
			tt.Fatal("the logo is not on screen")
		}
		return fmt.Sprint(buf.CellAt(logo.Visible.X, logo.Visible.Y).Style.Fg)
	}
	if plain, hovered := color(nil), color(logoButton{app: app}); plain == hovered {
		tt.Errorf("the logo is %s under the pointer, the same as without it", hovered)
	}
}

func TestSnapshotLogoMenu(tt *testing.T) {
	app := testApp()
	// Opening the menu requests focus for it, which the snapshot applies.
	app.openLogoMenu("")
	t.AssertSnapshotNamed(tt, "LogoMenu", app, snapW, snapH, "Links menu open above the Posting logo at the end of the footer")

	nerd := New(Config{
		Version:      "3.0.0-dev",
		Collection:   model.SampleCollection(),
		Environments: StaticEnvironments(model.SampleEnvironments()),
		UserHost:     "user@host",
		NerdFonts:    true,
	})
	nerd.openLogoMenu("")
	t.AssertSnapshotNamed(tt, "LogoMenu_nerd", nerd, snapW, snapH, "Links menu with Nerd Font icons")
}
