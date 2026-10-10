package ui

import (
	"fmt"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestSnapshotLongResponseHeadersScroll(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	resp := fixedResponse()
	for i := 1; i <= 40; i++ {
		resp.Headers = append(resp.Headers, model.Header{Name: fmt.Sprintf("X-Header-%d", i), Value: fmt.Sprintf("value %d", i)})
	}
	s := app.current()
	s.showResponse(resp, nil)
	s.phase.Set(exchangeDone)
	s.responseTab.Set("headers")
	for i := 0; i < 30; i++ {
		pressOn(tt, app, "resp-headers", "down")
	}
	if s.responseHeadersScroll.GetOffset() == 0 {
		tt.Fatal("moving the cursor below the panel didn't scroll the headers")
	}
	t.AssertSnapshot(tt, app, snapW, snapH, "Response headers longer than the panel: the table has scrolled to keep the cursor in view and, with focus elsewhere, still shows a scrollbar")
}

func TestSnapshotLongVariablesScroll(tt *testing.T) {
	app := testApp()
	vars := map[string]string{}
	for i := 1; i <= 40; i++ {
		vars[fmt.Sprintf("VAR_%02d", i)] = fmt.Sprintf("value %d", i)
	}
	app.sessionVars.Set(vars)
	app.openVariables()
	for i := 0; i < 30; i++ {
		pressOn(tt, app, "vars-table", "down")
	}
	if app.variables.scroll.GetOffset() == 0 {
		tt.Fatal("moving the cursor below the overlay didn't scroll the variables")
	}
	t.AssertSnapshot(tt, app, snapW, snapH, "Variables overlay with more variables than fit: the table has scrolled to the cursor and shows a scrollbar")
}
