package ui

import (
	"errors"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

// recordingStore remembers what the app asked it to save and delete.
type recordingStore struct {
	saved   []string
	deleted []string
	fail    error
}

func (r *recordingStore) Save(req model.Request) error {
	if r.fail != nil {
		return r.fail
	}
	r.saved = append(r.saved, req.File)
	return nil
}

func (r *recordingStore) Delete(file string) error {
	if r.fail != nil {
		return r.fail
	}
	r.deleted = append(r.deleted, file)
	return nil
}

func storeApp(store *recordingStore) *App {
	return New(Config{Collection: model.SampleCollection(), Store: store, UserHost: "user@host"})
}

func TestSaveWritesThroughTheStore(tt *testing.T) {
	store := &recordingStore{}
	app := storeApp(store)
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.url.SetText("https://changed.test/users")
	s.urlEdited()
	app.saveRequest()
	if len(store.saved) != 1 || store.saved[0] != "users/list-users.posting.yaml" {
		tt.Fatalf("saved = %v", store.saved)
	}
	if s.dirty.Peek() {
		tt.Fatal("saving should mark the tab clean")
	}

	app.newTab()
	app.saveRequest()
	app.save.name.SetText("Brand new")
	app.save.folder.SetText("misc")
	app.submitSave()
	if got := store.saved[len(store.saved)-1]; got != "misc/brand-new.posting.yaml" {
		tt.Fatalf("new request saved to %q", got)
	}
	if !app.fileExists("misc/brand-new.posting.yaml") {
		tt.Fatal("the new request should appear in the collection")
	}

	app.deleteRequests([]string{"misc/brand-new.posting.yaml"})
	if len(store.deleted) != 1 || app.fileExists("misc/brand-new.posting.yaml") {
		tt.Fatalf("deleted = %v", store.deleted)
	}
}

func TestFailedSaveKeepsTheTabDirty(tt *testing.T) {
	store := &recordingStore{fail: errors.New("disk full")}
	app := storeApp(store)
	app.openRequest(sampleRequest(tt, "List users"))
	s := app.current()
	s.touch()
	app.saveRequest()
	if !s.dirty.Peek() {
		tt.Fatal("a failed save must leave the tab dirty")
	}
	if toast := app.toast.Peek(); toast.kind != toastError {
		tt.Fatalf("toast = %+v, want an error", toast)
	}
}
