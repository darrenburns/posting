package ui

import (
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
