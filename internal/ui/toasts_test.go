package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
)

// assertToast checks notifications through the public widget, with enough
// width to assert message text independently of wrapping in the app.
func assertToast(tt *testing.T, app *App, message string) {
	tt.Helper()
	text := t.RenderToPlainString(t.Column{
		Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{t.Toasts{State: app.toasts, Width: snapW - 2}},
	}, snapW, snapH)
	if !strings.Contains(text, message) {
		tt.Fatalf("notification %q missing:\n%s", message, text)
	}
}

func TestNotificationsQueueWithoutTakingFocus(tt *testing.T) {
	app := testApp()
	tt.Cleanup(app.toasts.Clear)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click(urlInputID)
	for _, notice := range []struct {
		message string
		kind    t.ToastSeverity
	}{
		{"Saved request", toastSuccess},
		{"Environment reloaded", toastInfo},
		{"Missing variable", toastWarning},
		{"Could not save", toastError},
	} {
		app.notify(notice.message, notice.kind)
	}
	text := p.ScreenText()
	previous := -1
	for _, message := range []string{"Saved request", "Environment reloaded", "Missing variable", "+1 more"} {
		at := strings.Index(text, message)
		if at <= previous {
			tt.Fatalf("notifications missing or out of order at %q:\n%s", message, text)
		}
		previous = at
	}
	if strings.Contains(text, "Could not save") || app.toasts.Len() != 4 {
		tt.Fatalf("the fourth notification should wait behind three visible ones:\n%s", text)
	}
	if p.FocusedID() != urlInputID {
		tt.Fatal("notifications took keyboard focus")
	}
	card := p.TextOf("toasts-toast-1")
	for _, edge := range []string{"▊", "▎", "▔", "▁"} {
		if !strings.Contains(card, edge) {
			tt.Fatalf("notification is missing tall border %q:\n%s", edge, card)
		}
	}
	p.Click("toasts-toast-1")
	text = p.ScreenText()
	if strings.Contains(text, "Saved request") || strings.Contains(text, "+1 more") || !strings.Contains(text, "Could not save") || app.toasts.Len() != 3 {
		tt.Fatalf("dismissing a notification did not promote the next one:\n%s", text)
	}
	if p.FocusedID() != urlInputID {
		tt.Fatal("dismissing a notification took keyboard focus")
	}
	p.Type("https://example.test")
	if app.current().url.GetText() != "https://example.test" {
		tt.Fatal("notifications blocked typing in the request URL")
	}
}

func TestStartupReportsEveryProblem(tt *testing.T) {
	messages := []string{"Cannot load history", "Cannot read environment", "Invalid theme", "Cannot load request"}
	app := New(Config{UserHost: "user@host", StartupMessages: messages})
	tt.Cleanup(app.toasts.Clear)
	p := t.NewPilot(tt, app, snapW, snapH)
	if app.toasts.Len() != len(messages) {
		tt.Fatalf("queued %d startup problems, want %d", app.toasts.Len(), len(messages))
	}
	for _, message := range messages[:3] {
		if !strings.Contains(p.ScreenText(), message) {
			tt.Fatalf("startup problem %q missing", message)
		}
	}
	p.Click("toasts-toast-1")
	if !strings.Contains(p.ScreenText(), messages[3]) {
		tt.Fatal("the last startup problem was lost")
	}
}
