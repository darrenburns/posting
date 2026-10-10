package ui

import (
	"reflect"
	"testing"

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
