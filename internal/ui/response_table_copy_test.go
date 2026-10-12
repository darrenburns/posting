package ui

import (
	"slices"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

func tableApp(tt *testing.T, tab string) (*App, *Session) {
	tt.Helper()
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	resp := fixedResponse()
	resp.Cookies = append(resp.Cookies, model.Cookie{Name: "token", Value: "eyJhbGciOi.J9", Path: "/"})
	s.showResponse(resp, nil)
	s.phase.Set(exchangeDone)
	s.responseTab.Set(tab)
	return app, s
}

// copiedText is what pressing key copies from the row under the cursor.
func copiedText(tt *testing.T, s *Session, key string) string {
	tt.Helper()
	row, ok := s.cursorRow()
	if !ok {
		tt.Fatal("no row under the cursor")
	}
	for _, how := range rowCopies {
		if slices.Contains(how.keys, key) {
			return how.text(row)
		}
	}
	tt.Fatalf("no copy key %q", key)
	return ""
}

func TestHeaderRowCopyKeys(tt *testing.T) {
	app, s := tableApp(tt, "headers")
	pressOn(tt, app, "resp-headers", "down")
	for _, tc := range []struct{ key, text, toast string }{
		{"y", "example", "Copied the value of Server"},
		{"c", "example", "Copied the value of Server"},
		{"n", "Server", "Copied Server"},
		{"b", "Server: example", "Copied the Server header"},
	} {
		pressOn(tt, app, "resp-headers", tc.key)
		if got := app.toast.Peek().message; got != tc.toast {
			tt.Errorf("%s: toast %q, want %q", tc.key, got, tc.toast)
		}
		if got := copiedText(tt, s, tc.key); got != tc.text {
			tt.Errorf("%s copies %q, want %q", tc.key, got, tc.text)
		}
	}
}

func TestCookieRowCopiesAsACookieHeaderWould(tt *testing.T) {
	app, s := tableApp(tt, "cookies")
	pressOn(tt, app, "resp-cookies", "down")
	pressOn(tt, app, "resp-cookies", "b")
	if got := app.toast.Peek().message; got != "Copied the token cookie" {
		tt.Errorf("toast %q", got)
	}
	if got := copiedText(tt, s, "b"); got != "token=eyJhbGciOi.J9" {
		tt.Errorf("b copies %q", got)
	}
	if got := copiedText(tt, s, "y"); got != "eyJhbGciOi.J9" {
		tt.Errorf("y copies %q", got)
	}
}

func TestPaletteCopiesTheRowUnderTheTableCursor(tt *testing.T) {
	app, _ := tableApp(tt, "headers")
	labels := func() map[string]string {
		found := map[string]string{}
		for _, item := range app.paletteItems() {
			found[item.Label] = item.Description
		}
		return found
	}
	items := labels()
	for _, label := range []string{"Copy header value", "Copy header name", "Copy header name and value"} {
		if desc, ok := items[label]; !ok || desc != "Content-Type" {
			tt.Errorf("palette %q: present=%v description=%q, want the cursor row's name", label, ok, desc)
		}
	}
	app.current().responseTab.Set("body")
	if _, ok := labels()["Copy header value"]; ok {
		tt.Error("the palette offers to copy a header while the body is on show")
	}
}

func TestSnapshotResponseHeadersCopyHints(tt *testing.T) {
	app, _ := tableApp(tt, "headers")
	t.RequestFocus("resp-headers")
	pressOn(tt, app, "resp-headers", "down")
	pressOn(tt, app, "resp-headers", "b")
	t.AssertSnapshot(tt, app, snapW, snapH, "Response headers focused with the cursor on Server: the footer offers y Copy value, n Copy name and b Copy both, and a toast confirms the Server header was copied")
}
