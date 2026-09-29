package ui

import (
	t "github.com/darrenburns/terma"
)

const (
	logoID     = "footer-logo"
	logoMenuID = "footer-logo-menu"

	sponsorURL  = "https://github.com/sponsors/darrenburns"
	mastodonURL = "https://fosstodon.org/@darrenburns"
)

// footerLogo is the name and version at the end of the footer. Clicking it
// opens a menu of links: the docs, sponsorship and Mastodon.
type footerLogo struct{ app *App }

func (l footerLogo) Build(ctx t.BuildContext) t.Widget {
	a := l.app
	return t.Row{
		Style: t.Style{Padding: t.EdgeInsets{Left: 2}},
		Children: []t.Widget{
			logoButton{app: a, focused: ctx.FocusedSignal()},
			t.ShowWhen(a.logoMenuOpen.Get(), t.Menu{
				ID:        logoMenuID,
				State:     a.logoMenu,
				AnchorID:  logoID,
				Anchor:    t.AnchorTopRight,
				OnDismiss: a.closeLogoMenu,
			}),
		},
	}
}

// logoButton is the clickable text of the logo. It brightens under the
// pointer, and while its menu is open.
type logoButton struct {
	app     *App
	focused t.AnySignal[t.Focusable]
}

func (b logoButton) WidgetID() string { return logoID }

// OnClick opens the menu. The logo isn't focusable, so the menu remembers
// what was focused when it opened and hands focus back when it closes. The
// focus is read here rather than in Build, so the logo doesn't rebuild on
// every focus move. A click while the menu is open lands outside it, which
// closes it before reaching here.
func (b logoButton) OnClick(t.MouseEvent) {
	var returnFocus string
	if b.focused.IsValid() {
		returnFocus = focusedIDOf(b.focused.Peek())
	}
	b.app.openLogoMenu(returnFocus)
}

func (b logoButton) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	color := theme.TextDisabled
	if b.app.logoMenuOpen.Get() {
		color = theme.AccentText
	} else if ctx.IsHovered(b) {
		color = theme.Text
	}
	return t.Text{Content: "Posting " + b.app.version, Style: t.Style{ForegroundColor: color}}
}

func (a *App) openLogoMenu(returnFocus string) {
	if returnFocus == logoMenuID {
		returnFocus = ""
	}
	a.logoReturnFocus = returnFocus
	a.logoMenu.SetCursorIndex(0)
	a.logoMenuOpen.Set(true)
	t.RequestFocus(logoMenuID)
}

func (a *App) closeLogoMenu() {
	a.logoMenuOpen.Set(false)
	if a.logoReturnFocus != "" {
		t.RequestFocus(a.logoReturnFocus)
	}
}

func (a *App) logoMenuItems() []t.MenuItem {
	link := func(icon, label, hint, url string) t.MenuItem {
		return t.MenuItem{Label: icon + label, Shortcut: hint, Action: func() {
			a.closeLogoMenu()
			a.openLink(label, url)
		}}
	}
	return []t.MenuItem{
		link(a.icons.docs, "Docs", "posting.sh", docsURL),
		link(a.icons.donate, "Donate", "GitHub Sponsors", sponsorURL),
		link(a.icons.mastodon, "Mastodon", "@darrenburns@fosstodon.org", mastodonURL),
	}
}
