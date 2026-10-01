package ui

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
)

func TestDirectResponseCursorMoveRendersInFirstFrame(tt *testing.T) {
	text := strings.Repeat("ordinary line\n", 80) + "TARGET last line"
	app, session := bodyApp(tt, "text/plain", text)
	buffer := uv.NewBuffer(snapW, snapH)
	renderer := t.NewRenderer(buffer, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Update(app)
	session.responseBody.CursorIndex.Set(len(session.responseBody.Content.Peek()) - len("TARGET last line"))
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
	if !strings.Contains(visible.String(), "TARGET last line") {
		tt.Fatalf("first rendered frame after direct cursor move hides target (scroll offset %d):\n%s", session.responseBodyScroll.Offset.Peek(), visible.String())
	}
}
