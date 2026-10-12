package ui

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestJumpLabelsFollowRequestSessionAndKind(tt *testing.T) {
	app := testApp()
	httpSession := app.current()
	buffer := uv.NewBuffer(snapW, snapH)
	renderer := t.NewRenderer(buffer, snapW, snapH, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Update(app)
	app.newTab()
	renderer.Update(app)
	app.setKind(model.KindGraphQL)
	renderer.Update(app)
	graphqlSession := app.current()

	for _, step := range []struct {
		name    string
		session *Session
		labels  map[string]string
	}{
		{"new GraphQL request", graphqlSession, map[string]string{"gql-query": "i", "gql-variables": "o", "headers": "q"}},
		{"existing HTTP request", httpSession, map[string]string{"headers": "q", "body": "w"}},
		{"existing GraphQL request", graphqlSession, map[string]string{"gql-query": "i", "gql-variables": "o", "headers": "q"}},
	} {
		app.active.Set(step.session.id)
		renderer.Update(app)
		app.jump.Activate()
		renderer.Update(app)
		for tab, want := range step.labels {
			entry := renderer.WidgetByID(tabID(requestTabsID, tab))
			if entry == nil {
				tt.Fatalf("%s: missing %s tab", step.name, tab)
			}
			cell := buffer.CellAt(entry.Visible.X+1, entry.Visible.Y)
			if cell == nil || cell.Content != want {
				tt.Errorf("%s: %s jump label = %v, want %q", step.name, tab, cell, want)
			}
		}
		app.jump.Deactivate()
		renderer.Update(app)
	}
}
