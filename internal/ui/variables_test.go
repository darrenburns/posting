package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
)

func variableRow(t *testing.T, app *App, name string) model.Variable {
	t.Helper()
	for _, v := range app.variables.table.GetRows() {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("no variable row named %q", name)
	return model.Variable{}
}

func TestVariablesEditInPlace(t *testing.T) {
	app := testApp()
	app.openVariables()
	rows := app.variables.table.GetRows()
	if last := rows[len(rows)-1]; !isAddRow(last) {
		t.Fatalf("last row = %+v, want the add row", last)
	}

	app.editVariable(variableRow(t, app, "BASE_URL"))
	if got := app.variables.value.GetText(); got != "http://localhost:8000" {
		t.Fatalf("edit value = %q, want the current value", got)
	}
	app.variables.value.SetText("http://localhost:9000")
	if !app.commitVariable() {
		t.Fatal("commit failed")
	}
	if got := variableRow(t, app, "BASE_URL"); got.Value != "http://localhost:9000" || got.Source != "session" {
		t.Fatalf("BASE_URL = %+v, want a session override", got)
	}
	if v, _ := app.variables.table.SelectedRow(); v.Name != "BASE_URL" {
		t.Fatalf("cursor on %q after commit, want BASE_URL", v.Name)
	}
	if app.variables.editing.Peek() != "" {
		t.Fatal("still editing after commit")
	}

	// Committing an unchanged environment value doesn't create an override.
	app.editVariable(variableRow(t, app, "PASSWORD"))
	app.commitVariable()
	if got := variableRow(t, app, "PASSWORD"); got.Source != "local.env" {
		t.Fatalf("PASSWORD source = %q, want local.env", got.Source)
	}

	// Double-clicking another row mid-edit saves the edit before moving.
	app.editVariable(variableRow(t, app, "BASE_URL"))
	app.variables.value.SetText("http://localhost:9001")
	app.editVariable(variableRow(t, app, "API_TOKEN"))
	if variableRow(t, app, "BASE_URL").Value != "http://localhost:9001" || app.variables.editing.Peek() != "API_TOKEN" {
		t.Fatal("switching rows should save the first edit and start the second")
	}
	app.cancelVariableEdit()

	// Escape backs out of an edit before it closes the overlay.
	app.editVariable(variableRow(t, app, "API_TOKEN"))
	app.variables.value.SetText("discarded")
	app.dismissVariables()
	if app.overlay.Peek() != "variables" || variableRow(t, app, "API_TOKEN").Value != "dev-token-123" {
		t.Fatal("escape during an edit should cancel it and keep the overlay open")
	}
	app.dismissVariables()
	if app.overlay.Peek() != "" {
		t.Fatal("escape while browsing should close the overlay")
	}
}

func TestVariablesAddRow(t *testing.T) {
	app := testApp()
	app.openVariables()

	app.addVariable()
	app.variables.name.SetText("1BAD")
	app.variables.value.SetText("x")
	if app.commitVariable() {
		t.Fatal("commit accepted an invalid name")
	}
	if app.variables.err.Peek() == "" || app.variables.editing.Peek() != addRowKey {
		t.Fatal("an invalid name should keep the add row open with an error")
	}

	app.variables.name.SetText("HOST")
	if !app.commitVariable() {
		t.Fatal("commit failed")
	}
	if want := map[string]string{"HOST": "x"}; !reflect.DeepEqual(app.sessionVars.Peek(), want) {
		t.Fatalf("session vars = %v, want %v", app.sessionVars.Peek(), want)
	}
	if app.variables.err.Peek() != "" {
		t.Fatal("error should clear once the edit is saved")
	}

	// Leaving an untouched add row just stops editing.
	app.addVariable()
	app.commitAndMove(-1)
	if app.variables.editing.Peek() != "" || len(app.sessionVars.Peek()) != 1 {
		t.Fatal("an empty add row should be discarded")
	}
}

func TestGeneratedSendsAndResponseCaptures(t *testing.T) {
	type received struct{ path, id, body, token string }
	wire := make(chan received, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		id := r.Header.Get("Idempotency-Key")
		wire <- received{r.URL.Path, id, string(body), r.Header.Get("Authorization")}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/missing" {
			io.WriteString(w, `{"data":{"token":"must-not-overwrite"}}`)
			return
		}
		if r.URL.Path == "/ambiguous" {
			io.WriteString(w, `{"data":{"token":"must-not-overwrite","sessions":[{"id":"one","current":true},{"id":"two","current":true}]}}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"token": "${literal}$cash",
			"sessions": []any{
				map[string]any{"id": "not-current", "current": false},
				map[string]any{"id": id, "current": true},
			},
		}})
	}))
	defer server.Close()
	req, err := collection.ParseRequest([]byte(`name: Start session
method: POST
url: ${BASE}/sessions/${$uuid}
headers:
  - name: Idempotency-Key
    value: ${$uuid}
body:
  content: '${$uuid}'
captures: {SESSION_ID: '$.data.sessions[?(@.current == true)].id', API_TOKEN: data.token}
`), "session.posting.yaml")
	if err != nil {
		t.Fatal(err)
	}
	app := New(Config{})
	app.sender = client.NewHTTP("posting-test", client.TLSSettings{})
	app.sessionVars.Set(map[string]string{"BASE": server.URL, "API_TOKEN": "old"})
	app.openRequest(req)
	s := app.current()
	updates := make(chan func(), 64)
	s.dispatch = func(fn func()) { updates <- fn }
	finish := func() {
		t.Helper()
		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()
		for s.phase.Peek() == exchangeSending {
			select {
			case update := <-updates:
				update()
			case <-deadline.C:
				t.Fatal("send did not finish")
			}
		}
		if s.phase.Peek() != exchangeDone {
			t.Fatalf("send failed: %v", s.err.Peek())
		}
	}
	previous := ""
	for range 2 {
		app.send()
		finish()
		got := <-wire
		if !regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(got.id) || got.id == previous || got.path != "/sessions/"+got.id || got.body != got.id {
			t.Fatalf("generated value not fresh and consistent on the wire: %+v", got)
		}
		previous = got.id
		if app.sessionVars.Peek()["SESSION_ID"] != got.id || app.sessionVars.Peek()["API_TOKEN"] != "${literal}$cash" {
			t.Fatal("response wasn't captured into session variables")
		}
		if app.overlay.Peek() == "variables" && variableRow(t, app, "SESSION_ID").Value != got.id {
			t.Fatal("capture didn't refresh the open variables screen")
		}
		app.openVariables()
	}
	data, err := collection.MarshalRequest(s.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := collection.ParseRequest(data, req.File)
	if err != nil || !reflect.DeepEqual(loaded.Captures, req.Captures) || loaded.Headers[0].Value != "${$uuid}" {
		t.Fatalf("saving replaced generator or lost captures: %s, %v", data, err)
	}
	// Editing a snapshot must not change the saved capture definitions.
	snapshot := s.Snapshot()
	snapshot.Captures["SESSION_ID"] = "other"
	if s.Snapshot().Captures["SESSION_ID"] != "$.data.sessions[?(@.current == true)].id" {
		t.Fatal("snapshot shares capture definitions with editor")
	}
	next := model.NewRequest()
	next.URL = "${BASE}/sessions/${SESSION_ID}"
	next.Auth = model.Auth{Type: model.AuthBearer, Token: "${API_TOKEN}"}
	s.Load(next)
	app.send()
	finish()
	got := <-wire
	if got.path != "/sessions/"+previous || got.token != "Bearer ${literal}$cash" {
		t.Fatalf("next request didn't use literal captured values: %+v", got)
	}

	// A late completion uses its sent snapshot, even after the editor changes,
	// and never partially applies a failed capture set.
	for _, tc := range []struct{ path, warning string }{
		{"/missing", "matched 0 values"},
		{"/ambiguous", "matched 2 values"},
	} {
		s.Load(req)
		s.url.SetText(server.URL + tc.path)
		app.send()
		s.Load(next)
		app.openVariables()
		before := app.sessionVars.Peek()
		finish()
		<-wire
		if !reflect.DeepEqual(app.sessionVars.Peek(), before) {
			t.Fatalf("%s: failed capture changed session values", tc.path)
		}
		assertToast(t, app, tc.warning)
		if !strings.Contains(string(s.response.Peek().Body), "must-not-overwrite") {
			t.Fatal("capture failure hid response")
		}
		if variableRow(t, app, "SESSION_ID").Value != previous {
			t.Fatal("variables screen lost the last successful capture")
		}
	}
	if len(app.history.Peek()) != 5 {
		t.Fatal("capture failure lost history")
	}
}
