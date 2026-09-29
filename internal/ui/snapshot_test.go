package ui

import (
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const snapW, snapH = 130, 36

func testApp() *App {
	return New(Config{
		Version:      "3.0.0-dev",
		Collection:   model.SampleCollection(),
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
	})
}

func sampleRequest(t *testing.T, name string) model.Request {
	t.Helper()
	var found *model.Request
	model.SampleCollection().Walk(func(_ *model.Collection, r model.Request) {
		if r.Name == name {
			found = &r
		}
	})
	if found == nil {
		t.Fatalf("no sample request named %q", name)
	}
	return *found
}

func fixedResponse() *model.Response {
	return &model.Response{
		StatusCode: 200,
		Reason:     "OK",
		Proto:      "HTTP/1.1",
		Method:     model.MethodGet,
		URL:        "http://localhost:8000/users?page=1&per_page=20",
		Headers: []model.Header{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "Server", Value: "example"},
		},
		Cookies: []model.Cookie{{Name: "session", Value: "abc123", Path: "/", HTTPOnly: true}},
		Body:    []byte(`{"users": [{"id": 1, "name": "Ada", "admin": true}], "page": 1, "next": null}`),
		Elapsed: 42 * time.Millisecond,
		Trace: []model.TraceEvent{
			{Stage: model.TraceConnect, State: model.TraceComplete, Duration: 8 * time.Millisecond},
			{Stage: model.TraceTLS, State: model.TraceSkipped},
			{Stage: model.TraceSendHeaders, State: model.TraceComplete, Duration: 2 * time.Millisecond},
			{Stage: model.TraceSendBody, State: model.TraceComplete, Duration: 1 * time.Millisecond},
			{Stage: model.TraceReceiveHeaders, State: model.TraceComplete, Duration: 24 * time.Millisecond},
			{Stage: model.TraceReceiveBody, State: model.TraceComplete, Duration: 6 * time.Millisecond},
			{Stage: model.TraceClosed, State: model.TraceComplete, Duration: 1 * time.Millisecond},
		},
	}
}

func TestSnapshotEmpty(tt *testing.T) {
	t.AssertSnapshot(tt, testApp(), snapW, snapH, "Fresh start: collection tree, blank request tab, empty response")
}

func TestSnapshotResponse(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	t.AssertSnapshot(tt, app, snapW, snapH, "List users opened from the collection with a highlighted JSON response")
}

func TestSnapshotJump(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	app.jump.Activate()
	t.AssertSnapshot(tt, app, snapW, snapH, "Jump mode: fixed keys on the URL bar and tabs, hints on requests, the open tab and fields")
}

func TestSnapshotRequestTabs(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "Create user"))
	s := app.current()
	for _, tab := range []string{"body", "auth", "options"} {
		s.requestTab.Set(tab)
		t.AssertSnapshotNamed(tt, "RequestTabs_"+tab, app, snapW, snapH, "Create user with the "+tab+" tab selected")
	}
	login := testApp()
	login.openRequest(sampleRequest(tt, "Login"))
	login.current().requestTab.Set("body")
	t.AssertSnapshotNamed(tt, "RequestTabs_form", login, snapW, snapH, "Form-encoded body editor")
}

func TestSnapshotHorizontalWithHistory(tt *testing.T) {
	app := testApp()
	app.layout.Set(layoutHorizontal)
	app.sidebarTab.Set("history")
	sent := time.Date(2026, 9, 28, 14, 30, 0, 0, time.UTC)
	var history []model.HistoryEntry
	for i, name := range []string{"List users", "Create user", "Get user"} {
		req := sampleRequest(tt, name)
		resp := fixedResponse()
		resp.Method = req.Method
		if i == 2 {
			resp.StatusCode, resp.Reason = 404, "Not Found"
		}
		history = append(history, model.HistoryEntry{ID: int64(i + 1), Request: req, Response: resp, SentAt: sent.Add(-time.Duration(i) * time.Minute)})
	}
	app.history.Set(history)
	app.historyList.SetItems(history)
	app.openHistory(history[0])
	app.current().responseTab.Set("trace")
	t.AssertSnapshot(tt, app, snapW, snapH, "Side-by-side layout, history sidebar, response loaded from history showing the trace waterfall")
}

func TestSnapshotOverlays(tt *testing.T) {
	app := testApp()
	app.overlay.Set("help")
	t.AssertSnapshotNamed(tt, "Overlay_help", app, snapW, snapH, "Keyboard shortcuts overlay")

	app = testApp()
	app.openVariables()
	t.AssertSnapshotNamed(tt, "Overlay_variables", app, snapW, snapH, "Variables overlay with secrets masked")

	app = testApp()
	app.openVariables()
	app.editVariable(app.variables.table.GetRows()[0])
	t.AssertSnapshotNamed(tt, "Overlay_variables_edit", app, snapW, snapH, "Variables overlay editing the first value in place")

	app = testApp()
	app.openVariables()
	app.addVariable()
	t.AssertSnapshotNamed(tt, "Overlay_variables_add", app, snapW, snapH, "Variables overlay adding a variable in the table's last row")

	app = testApp()
	app.save.prefill(model.NewRequest(), "users")
	app.overlay.Set("save")
	t.AssertSnapshotNamed(tt, "Overlay_save", app, snapW, snapH, "Save request dialog")
}
