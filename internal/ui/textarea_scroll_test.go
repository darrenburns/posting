package ui

import (
	"fmt"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

// longResponse has a body far taller than the response panel.
func longResponse() *model.Response {
	resp := fixedResponse()
	var items []string
	for i := 1; i <= 80; i++ {
		items = append(items, fmt.Sprintf(`{"id": %d, "name": "User %d"}`, i, i))
	}
	resp.Body = []byte("[" + strings.Join(items, ",") + "]")
	return resp
}

func TestSnapshotLongResponseScrollsToCursor(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(longResponse(), nil)
	s.phase.Set(exchangeDone)
	// Page down well below the visible lines, as the keyboard would.
	for i := 0; i < 8; i++ {
		pressOn(tt, app, "resp-body", "pgdown")
	}
	t.AssertSnapshot(tt, app, snapW, snapH, "Long response body: the cursor has moved far down and the body has scrolled to keep it in view, with a scrollbar")
}

func TestSnapshotLongRequestBodyScrolls(tt *testing.T) {
	app := testApp()
	req := sampleRequest(tt, "Create user")
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf(`  "field_%d": %d,`, i, i))
	}
	req.Body.Raw = "{\n" + strings.Join(lines, "\n") + "\n  \"last\": true\n}"
	app.openRequest(req)
	s := app.current()
	s.requestTab.Set("body")
	for i := 0; i < 70; i++ {
		pressOn(tt, app, "req-body-text", "down")
	}
	t.AssertSnapshot(tt, app, snapW, snapH, "Long request body with the cursor at the end: the editor has scrolled to the last line")
}
