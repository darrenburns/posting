package ui

import (
	"context"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

// bodyApp shows a response whose body is text, with focus in the body.
func bodyApp(tt *testing.T, contentType, text string) (*App, *Session) {
	tt.Helper()
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	resp := fixedResponse()
	resp.Headers = []model.Header{{Name: "Content-Type", Value: contentType}}
	resp.Body = []byte(text)
	s.showResponse(resp, nil)
	s.phase.Set(exchangeDone)
	return app, s
}

func press(tt *testing.T, app *App, keys ...string) {
	tt.Helper()
	for _, key := range keys {
		pressOn(tt, app, "resp-body", key)
	}
}

func bodyKeys(tt *testing.T, app *App) map[string]bool {
	tt.Helper()
	renderer := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	keys := map[string]bool{}
	for _, entry := range renderer.Render(app) {
		if entry.ID == "resp-body" {
			for _, kb := range entry.Focusable.(t.KeybindProvider).Keybinds() {
				keys[kb.Key] = true
			}
		}
	}
	return keys
}

func cursorAt(s *Session) string {
	return cursorPosition(s.responseBody.Content.Peek(), s.responseBody.CursorIndex.Peek())
}

func TestVisualModeCopiesTheCharacterUnderTheCursor(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
	press(tt, app, "v", "w")
	if text, _ := s.bodyCopyText(); text != "hello w" {
		tt.Fatalf("v w copies %q, want %q as in Vim", text, "hello w")
	}
	press(tt, app, "y")
	if s.responseVisual.Peek() || s.responseBody.HasSelection() {
		tt.Fatal("copying should leave visual mode and drop the selection")
	}
	if got := app.toast.Peek().message; got != "Copied 7 characters" {
		tt.Fatalf("toast %q", got)
	}
}

func TestVisualModeCopiesWhatIsHighlightedWhenSelectingBackwards(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
	press(tt, app, "w", "v", "b")
	if text, _ := s.bodyCopyText(); text != "hello w" {
		tt.Fatalf("w v b copies %q, want the highlighted %q", text, "hello w")
	}
}

func TestVisualModeExtendsSelectionAcrossLines(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world\nsecond line")
	press(tt, app, "v", "j", "$")
	if text, _ := s.bodyCopyText(); text != "hello world\nsecond line" {
		tt.Fatalf("v j $ copies %q", text)
	}
}

func TestShiftSelectionExcludesTheCursor(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world")
	press(tt, app, "L", "L", "shift+right", "L", "L")
	if text, _ := s.bodyCopyText(); text != "hello" {
		tt.Fatalf("five shift-moves copy %q, want %q", text, "hello")
	}
	press(tt, app, "l")
	if s.responseBody.HasSelection() {
		tt.Fatal("moving without shift outside visual mode should drop the selection")
	}
}

func TestCopyWithoutSelectionCopiesTheWholeBody(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello world")
	press(tt, app, "l", "y")
	if text, selected := s.bodyCopyText(); selected || text != "hello world" {
		tt.Fatalf("copy without a selection gave %q (selected=%v)", text, selected)
	}
	if got := app.toast.Peek().message; got != "Copied response body" {
		tt.Fatalf("toast %q", got)
	}
}

func TestVisualModeWithoutMovingCopiesOneCharacter(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello")
	press(tt, app, "v")
	if text, selected := s.bodyCopyText(); !selected || text != "h" {
		tt.Fatalf("v y copies %q (selected=%v), want %q", text, selected, "h")
	}
}

func TestLineStartIsSmartHome(tt *testing.T) {
	app, s := bodyApp(tt, "application/json", `{"users": [1]}`)
	press(tt, app, "j", "$")
	if got := cursorAt(s); got != "2:13" {
		tt.Fatalf("$ on the second line: %s", got)
	}
	press(tt, app, "0")
	if got := cursorAt(s); got != "2:3" {
		tt.Fatalf("first 0 should stop at the indented text, got %s", got)
	}
	press(tt, app, "^")
	if got := cursorAt(s); got != "2:1" {
		tt.Fatalf("second home should reach column 1, got %s", got)
	}
	press(tt, app, "home")
	if got := cursorAt(s); got != "2:3" {
		tt.Fatalf("home from column 1 should go to the text, got %s", got)
	}
}

func TestTopBottomAndMatchingBracket(tt *testing.T) {
	app, s := bodyApp(tt, "application/json", `{"users": [{"id": 1}], "page": 1}`)
	press(tt, app, "G")
	if got := cursorAt(s); got != "8:1" {
		tt.Fatalf("G: %s", got)
	}
	press(tt, app, "g")
	if got := cursorAt(s); got != "1:1" {
		tt.Fatalf("g: %s", got)
	}
	press(tt, app, "%")
	if got := cursorAt(s); got != "8:1" {
		tt.Fatalf("%% on the opening brace should reach the closing one, got %s", got)
	}
	press(tt, app, "%")
	if got := cursorAt(s); got != "1:1" {
		tt.Fatalf("%% back: %s", got)
	}
	// From the start of `  "users": [`, % finds the [ later on the line.
	press(tt, app, "j", "0", "%")
	if got := cursorAt(s); got != "6:3" {
		tt.Fatalf("%% from before a bracket: %s", got)
	}
}

func TestEscapeIsOnlyTakenInVisualMode(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello")
	if bodyKeys(tt, app)["escape"] {
		tt.Fatal("escape should reach the app (to cancel a request) outside visual mode")
	}
	press(tt, app, "v", "l")
	if !bodyKeys(tt, app)["escape"] {
		tt.Fatal("escape should leave visual mode")
	}
	press(tt, app, "escape")
	if s.responseVisual.Peek() || s.responseBody.HasSelection() {
		tt.Fatal("escape should leave visual mode and drop the selection")
	}
}

func TestSelectLineLeavesVisualMode(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello\nworld")
	press(tt, app, "v", "V")
	if text, _ := s.bodyCopyText(); text != "hello\n" || s.responseVisual.Peek() {
		tt.Fatalf("V copies %q (visual=%v)", text, s.responseVisual.Peek())
	}
	press(tt, app, "f7")
	if text, _ := s.bodyCopyText(); text != "hello\nworld" {
		tt.Fatalf("f7 copies %q", text)
	}
}

func TestNewResponseLeavesVisualMode(tt *testing.T) {
	app, s := bodyApp(tt, "text/plain", "hello")
	press(tt, app, "v", "l")
	s.showResponse(fixedResponse(), nil)
	if s.responseVisual.Peek() || s.responseBody.HasSelection() {
		tt.Fatal("a new response should start outside visual mode")
	}
}

func TestBracketMatchKeepsSyntaxColour(tt *testing.T) {
	body := t.NewTextAreaState("[1]")
	body.CursorIndex.Set(0)
	red := t.Hex("#ff0000")
	base := t.HighlighterFunc(func(text string, graphemes []string) []t.TextHighlight {
		return []t.TextHighlight{{Start: 0, End: len(graphemes), Style: t.SpanStyle{Foreground: red}}}
	})
	styles := map[int]t.SpanStyle{}
	for _, h := range withBracketMatch(base, body).Highlight("[1]", []string{"[", "1", "]"}) {
		for i := h.Start; i < h.End; i++ {
			styles[i] = h.Style
		}
	}
	for _, i := range []int{0, 2} {
		if !styles[i].Bold || styles[i].Underline != t.UnderlineSingle || styles[i].Foreground != red {
			tt.Fatalf("bracket %d: %+v", i, styles[i])
		}
	}
	if styles[1].Bold {
		tt.Fatal("only the brackets should be emphasised")
	}
}

func TestJumpsScrollTheNextFrame(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(longResponse(), nil)
	s.phase.Set(exchangeDone)
	buf := uv.NewBuffer(snapW, snapH)
	renderer := t.NewRenderer(buf, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	frame := func() string {
		renderer.Render(app)
		var lines []string
		for y := 0; y < snapH; y++ {
			lines = append(lines, buf.Line(y).String())
		}
		return strings.Join(lines, "\n")
	}
	frame()
	for _, step := range []struct{ key, want string }{
		{"G", `"name": "User 80"`},
		{"g", `"name": "User 1"`},
		{"V", `"name": "User 1"`},
		{"f7", `"name": "User 80"`},
	} {
		press(tt, app, step.key)
		if !strings.Contains(frame(), step.want) {
			tt.Fatalf("the first frame after %s doesn't show %s", step.key, step.want)
		}
	}
}

func TestSnapshotResponseVisualMode(tt *testing.T) {
	app, _ := bodyApp(tt, "application/json", `{"users": [{"id": 1, "name": "Ada"}], "page": 1}`)
	t.RequestFocus("resp-body")
	press(tt, app, "j", "j", "v", "j", "j", "$")
	t.AssertSnapshot(tt, app, snapW, snapH, "Response body in visual mode: lines 3 to 5 selected, a VISUAL badge and the cursor position 5:20 in the bar under the body")
}

func TestResponseBodySearch(tt *testing.T) {
	text := "Café.* first\n" + strings.Repeat("not a match\n", 80) + "CAFÉ.* last"
	app, s := bodyApp(tt, "text/plain", text)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body")
	p.Press("g", "v", "ctrl+f")
	if p.FocusedID() != responseSearchID || s.responseVisual.Peek() {
		tt.Fatalf("find should leave visual mode and focus search: focus=%q visual=%v", p.FocusedID(), s.responseVisual.Peek())
	}
	p.Type("café.*")
	check := func(count, selected, visible string) {
		tt.Helper()
		if got := strings.TrimSpace(p.TextOf(responseSearchID + "-count")); got != count {
			tt.Fatalf("search count = %q, want %q", got, count)
		}
		if got := s.responseBody.GetSelectedText(); got != selected {
			tt.Fatalf("selected %q, want %q", got, selected)
		}
		if !strings.Contains(p.TextOf("resp-body"), visible) {
			tt.Fatalf("search did not reveal %q", visible)
		}
	}
	check("1/2", "Café.*", "first")
	p.Press("shift+enter")
	check("2/2", "CAFÉ.*", "last")
	p.Press("enter")
	check("1/2", "Café.*", "first")
	p.Press("up")
	if p.FocusedID() != "resp-body" {
		tt.Fatal("up from search should focus the body")
	}
	p.Press("n")
	check("2/2", "CAFÉ.*", "last")
	p.Press("N")
	check("1/2", "Café.*", "first")
	p.Press("v", "n")
	check("2/2", "CAFÉ.*", "last")
	p.Press("escape")
	if p.FocusedID() != "resp-body" || s.responseBody.SearchQuery.Peek() != "" || s.responseSearchOpen.Peek() {
		tt.Fatal("escape should close search, clear highlights and return to the body")
	}
	p.Press("/")
	if s.responseSearch.GetSelectedText() != "café.*" {
		tt.Fatal("reopening search should select the previous query for replacement")
	}
	check("2/2", "CAFÉ.*", "last")
	p.Type("absent")
	p.Press("enter", "shift+enter")
	check("No matches", "", "last")
	p.Press("ctrl+a", "backspace")
	if s.responseBody.SearchQuery.Peek() != "" || len(s.responseBody.SearchMatches()) != 0 {
		tt.Fatal("clearing the input should clear the search")
	}
	p.Type("café.*")
	p.Press("up", "v", "n", "y")
	if got := p.Clipboard(); got != "CAFÉ.*" {
		tt.Fatalf("copying a search match after visual mode copied %q", got)
	}
	if got := s.responseBody.GetText(); got != text {
		tt.Fatal("searching modified the read-only body")
	}
}

func TestResponseBodySearchStaysWithItsSession(tt *testing.T) {
	app, first := bodyApp(tt, "text/plain", "one two one")
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body")
	p.Press("g", "/")
	p.Type("one")
	second := app.openSession(sampleRequest(tt, "Create user"))
	second.showResponse(fixedResponse(), nil)
	second.phase.Set(exchangeDone)
	t.RequestFocus("resp-body")
	p.Advance(0)
	if second.responseSearchOpen.Peek() || second.responseBody.SearchQuery.Peek() != "" {
		tt.Fatal("search leaked into another session")
	}
	p.Press("/", "escape", "alt+left")
	if app.current() != first {
		tt.Fatal("did not return to the first session")
	}
	t.RequestFocus("resp-body")
	p.Advance(0)
	p.Press("n")
	if got := strings.TrimSpace(p.TextOf(responseSearchID + "-count")); got != "2/2" {
		tt.Fatalf("session lost its search: %q", got)
	}
	// A replacement response must search the new body, not cached old matches.
	resp := fixedResponse()
	resp.Headers = []model.Header{{Name: "Content-Type", Value: "text/plain"}}
	resp.Body = []byte("a new one")
	first.showResponse(resp, nil)
	p.Advance(0)
	p.Press("n")
	if got := strings.TrimSpace(p.TextOf(responseSearchID + "-count")); got != "1/1" {
		tt.Fatalf("new response count = %q, want 1/1", got)
	}
	p.Press("y")
	if got := p.Clipboard(); got != "one" {
		tt.Fatalf("new response match copied %q", got)
	}
	// Search consumes Escape before the app's request cancellation binding.
	first.phase.Set(exchangeSending)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first.cancel = cancel
	p.Advance(0)
	p.Press("escape")
	if ctx.Err() != nil {
		tt.Fatal("closing search cancelled the request")
	}
	p.Press("escape")
	if ctx.Err() != context.Canceled {
		tt.Fatal("escape after closing search should still cancel the request")
	}
}

func TestSnapshotResponseSearch(tt *testing.T) {
	app, _ := bodyApp(tt, "application/json", `{"users": [{"name": "Ada"}, {"name": "Grace"}, {"name": "ADA"}]}`)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body")
	p.Press("g", "ctrl+f")
	p.Type("name")
	p.AssertSnapshot("ResponseSearch_matches", "Live response search: three highlighted matches, the first selected, with focus in the search input")
	p.Resize(82, snapH)
	p.Press("ctrl+f")
	p.Type("missing")
	p.AssertSnapshot("ResponseSearch_empty", "A narrow response pane shows No matches while keeping the search input and close control visible")
}
