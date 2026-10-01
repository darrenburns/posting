package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// clickText renders app, finds the nth occurrence (from 0) of text on
// screen and clicks its first cell the way the app routes mouse clicks: to
// the innermost widget under the pointer.
func clickText(tt *testing.T, app *App, text string, nth int, clicks ...int) {
	tt.Helper()
	count := 1
	if len(clicks) > 0 {
		count = clicks[0]
	}
	pressText(tt, app, text, nth, count, 0)
}

// shiftClickText is clickText with shift held.
func shiftClickText(tt *testing.T, app *App, text string, nth int) {
	tt.Helper()
	pressText(tt, app, text, nth, 1, uv.ModShift)
}

func pressText(tt *testing.T, app *App, text string, nth, count int, mod uv.KeyMod) {
	tt.Helper()
	buf := uv.NewBuffer(snapW, snapH)
	renderer := t.NewRenderer(buf, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(app)
	seen := 0
	for y := 0; y < snapH; y++ {
		var cells []string
		for x := 0; x < snapW; x++ {
			c := buf.CellAt(x, y)
			if c == nil || c.Content == "" {
				cells = append(cells, " ")
				continue
			}
			cells = append(cells, c.Content)
		}
		line := strings.Join(cells, "")
		for from := 0; ; {
			i := strings.Index(line[from:], text)
			if i < 0 {
				break
			}
			byteX := from + i
			if seen == nth {
				x := len([]rune(line[:byteX]))
				entry := renderer.WidgetAt(x, y)
				if entry == nil {
					tt.Fatalf("nothing at %q (%d,%d)", text, x, y)
				}
				event := t.MouseEvent{X: x, Y: y, LocalX: x - entry.Bounds.X, LocalY: y - entry.Bounds.Y, Button: uv.MouseLeft, Mod: mod, ClickCount: count}
				handled := false
				if down, ok := entry.EventWidget.(t.MouseDownHandler); ok {
					down.OnMouseDown(event)
					handled = true
				}
				// Like the app, also tell the widget that owns the pressed
				// part (a Tree for one of its rows).
				if owner := renderer.PointerOwnerAt(x, y); owner != nil && owner.ID != entry.ID {
					if down, ok := owner.EventWidget.(t.MouseDownHandler); ok {
						ownerEvent := event
						ownerEvent.LocalX, ownerEvent.LocalY = x-owner.Bounds.X, y-owner.Bounds.Y
						down.OnMouseDown(ownerEvent)
						handled = true
					}
				}
				if clickable, ok := entry.EventWidget.(t.Clickable); ok {
					clickable.OnClick(event)
					handled = true
				}
				if !handled {
					tt.Fatalf("clicking %q reaches %T (%q), which doesn't handle clicks", text, entry.EventWidget, entry.ID)
				}
				return
			}
			seen++
			from = byteX + len(text)
		}
	}
	tt.Fatalf("%q (occurrence %d) is not on screen", text, nth)
}

func TestClickBodyAndAuthChoices(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.requestTab.Set("body")
	clickText(tt, app, "Raw", 0)
	if s.bodyType.Peek() != model.BodyRaw {
		tt.Fatalf("body type = %s", s.bodyType.Peek())
	}
	clickText(tt, app, "XML", 0)
	if s.contentType.Peek() != "application/xml" {
		tt.Fatalf("content type = %s", s.contentType.Peek())
	}
	s.requestTab.Set("auth")
	clickText(tt, app, "Bearer token", 0)
	if s.authType.Peek() != model.AuthBearer {
		tt.Fatalf("auth = %s", s.authType.Peek())
	}
}

func TestClickRowToggleAndRemove(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	clickText(tt, app, "☑", 0)
	if got := s.headers.Values(); len(got) != 1 || got[0].Enabled {
		tt.Fatalf("header should be disabled: %+v", got)
	}
	clickText(tt, app, "✕", 1) // The first ✕ closes the request tab.
	if got := s.headers.Values(); len(got) != 0 {
		tt.Fatalf("header should be removed: %+v", got)
	}
}

func TestClickRequestTabsStrip(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	clickText(tt, app, " + ", 0)
	if len(app.sessions.Peek()) != 2 {
		tt.Fatalf("clicking + should open a tab; have %d", len(app.sessions.Peek()))
	}
	clickText(tt, app, "List users", 0)
	if app.current().file.Peek() != "users/list-users.posting.yaml" {
		tt.Fatal("clicking a tab should switch to it")
	}
	clickText(tt, app, "✕", 0)
	if len(app.sessions.Peek()) != 1 {
		tt.Fatal("clicking ✕ should close the tab")
	}
}

func TestClickResponseControls(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	clickText(tt, app, "wrap on", 0)
	if s.responseBody.WrapMode.Peek() != t.WrapNone {
		tt.Fatal("clicking wrap should turn wrapping off")
	}
	clickText(tt, app, "Trace", 0)
	if s.responseTab.Peek() != "trace" {
		tt.Fatalf("response tab = %s", s.responseTab.Peek())
	}
}

func TestClickEnvironmentOpensSwitcher(tt *testing.T) {
	app := testApp()
	clickText(tt, app, "local", 0)
	if !app.palette.Visible.Peek() {
		tt.Fatal("clicking the environment should open the switcher")
	}
}

func TestClickHistoryOpensEntry(tt *testing.T) {
	app := testApp()
	app.sidebarTab.Set("history")
	resp := fixedResponse()
	resp.StatusCode, resp.Reason = 404, "Not Found"
	entries := []model.HistoryEntry{{ID: 1, Request: sampleRequest(tt, "Get user"), Response: resp, SentAt: time.Date(2026, 9, 28, 14, 30, 0, 0, time.UTC)}}
	app.history.Set(entries)
	app.historyList.SetItems(entries)
	clickText(tt, app, "404", 0)
	if r := app.current().response.Peek(); r == nil || r.StatusCode != 404 {
		tt.Fatal("clicking a history entry should open it")
	}
}

func TestClickTreeOpensRequest(tt *testing.T) {
	app := testApp()
	clickText(tt, app, "Create user", 0)
	if s := app.current(); s.file.Peek() != "users/create-user.posting.yaml" {
		tt.Fatalf("clicking a request in the tree should open it; open file = %q", s.file.Peek())
	}
}

func TestShiftClickSelectsARangeOfRequests(tt *testing.T) {
	app := testApp()
	clickText(tt, app, "Get user", 0)
	opened := app.current().file.Peek()
	shiftClickText(tt, app, "Create user", 0)
	var names []string
	for _, r := range app.treeTargets() {
		names = append(names, r.Name)
	}
	if want := []string{"Get user", "List users", "Create user"}; !slices.Equal(names, want) {
		tt.Fatalf("shift+click should select from the clicked request to this one; selected %q, want %q", names, want)
	}
	if got := app.current().file.Peek(); got != opened {
		tt.Fatalf("shift+click should only select, but it opened %q", got)
	}
}

func TestSnapshotResponseTabs(tt *testing.T) {
	for _, tab := range []string{"headers", "cookies"} {
		app := testApp()
		app.openRequest(sampleRequest(tt, "List users"))
		s := app.current()
		resp := fixedResponse()
		resp.Headers = append(resp.Headers, model.Header{Name: "Access-Control-Allow-Credentials", Value: "true"})
		s.showResponse(resp, nil)
		s.phase.Set(exchangeDone)
		s.responseTab.Set(tab)
		t.AssertSnapshotNamed(tt, "ResponseTabs_"+tab, app, snapW, snapH, "Response "+tab+" table, with the name column fitted to the longest name")
	}
}
