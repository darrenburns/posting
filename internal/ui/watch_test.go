package ui

import (
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestReloadedCollectionUpdatesCleanTabsOnly(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	clean := app.current()
	app.openSession(sampleRequest(tt, "Get user"))
	dirty := app.current()
	dirty.url.SetText("https://edited.test")
	dirty.urlEdited()

	root := model.SampleCollection()
	root.Walk(func(folder *model.Collection, r model.Request) {
		for i := range folder.Requests {
			if folder.Requests[i].File == "users/list-users.posting.yaml" || folder.Requests[i].File == "users/get-user.posting.yaml" {
				folder.Requests[i].Description = "changed on disk"
			}
		}
	})
	app.replaceCollection(root)

	if got := clean.Snapshot().Description; got != "changed on disk" {
		tt.Errorf("clean tab description = %q", got)
	}
	if got := dirty.url.GetText(); got != "https://edited.test" {
		tt.Errorf("unsaved edits were replaced: URL = %q", got)
	}
	if app.collection.Peek() != root {
		tt.Error("the collection wasn't replaced")
	}
}

func TestSavingDoesNotLookLikeAChangeOnDisk(tt *testing.T) {
	app := testApp()
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	// The sample keeps its query in the table only, as saved files do; the
	// editor has it in the URL too.
	if !strings.Contains(s.url.GetText(), "?") {
		tt.Fatal("the editor should show the query in the URL")
	}
	app.toast.Set(toast{})
	app.replaceCollection(model.SampleCollection())
	if app.toast.Peek().message != "" {
		tt.Fatalf("an unchanged file was reloaded: %q", app.toast.Peek().message)
	}
}
