package ui

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	uv "github.com/darrenburns/ultraviolet"
)

func TestDirectResponseCursorMoveRendersInFirstFrame(tt *testing.T) {
	text := "TARGET first line\n" + strings.Repeat("ordinary line\n", 79) + "TARGET last line"
	app, session := bodyApp(tt, "text/plain", text)
	buffer := uv.NewBuffer(snapW, snapH)
	renderer := t.NewRenderer(buffer, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Update(app)
	for _, target := range []struct {
		index int
		text  string
	}{
		{len(session.responseBody.Content.Peek()) - len("TARGET last line"), "TARGET last line"},
		{0, "TARGET first line"},
	} {
		session.responseBody.CursorIndex.Set(target.index)
		renderer.Update(app)
		bounds := renderer.WidgetByID("resp-body").Visible
		var visible strings.Builder
		for y := bounds.Y; y < bounds.Y+bounds.Height; y++ {
			for x := bounds.X; x < bounds.X+bounds.Width; x++ {
				if cell := buffer.CellAt(x, y); cell != nil {
					visible.WriteString(cell.Content)
				}
			}
			visible.WriteByte('\n')
		}
		if !strings.Contains(visible.String(), target.text) {
			tt.Fatalf("first rendered frame after direct cursor move hides %q (scroll offset %d):\n%s", target.text, session.responseBodyScroll.Offset.Peek(), visible.String())
		}
	}
}

func TestResponseCursorRevealPreservesManualScrolling(tt *testing.T) {
	app, session := bodyApp(tt, "text/plain", strings.Repeat("ordinary line\n", 80))
	renderer := t.NewRenderer(uv.NewBuffer(snapW, snapH), snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Update(app)
	session.responseBody.CursorIndex.Set(len(session.responseBody.Content.Peek()))
	renderer.Update(app)
	session.responseBodyScroll.ScrollUp(20)
	want := session.responseBodyScroll.Offset.Peek()
	renderer.Update(app)
	renderer.Render(app)
	if got := session.responseBodyScroll.Offset.Peek(); got != want {
		tt.Fatalf("render moved manually scrolled viewport from %d to %d without a cursor move", want, got)
	}
}
