package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/darrenburns/posting/v3/internal/remote"
)

func TestNewRequestPreservesUnloadedFiles(t *testing.T) {
	for _, action := range []string{"remote", "dialog", "duplicate"} {
		for _, when := range []string{"invalid at load", "created after load"} {
			t.Run(action+"/"+when, func(t *testing.T) {
				h := newRemoteHarness(t)
				dir := collection.Dir{Root: t.TempDir()}
				file := "existing.posting.yaml"
				if action == "duplicate" {
					file = "existing-copy.posting.yaml"
				}
				target := filepath.Join(dir.Root, file)
				original := []byte("name: Existing\nurl: https://old.example\n")
				if when == "invalid at load" {
					original = []byte("name: [unclosed\n")
					if err := os.WriteFile(target, original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				root, _ := dir.Load()
				h.app.collection.Set(root)
				h.app.store = dir
				if err := os.WriteFile(target, original, 0o600); err != nil {
					t.Fatal(err)
				}
				req := model.NewRequest()
				req.Name, req.URL = "Existing", "https://new.example"
				h.app.openRequest(req)
				s := h.app.current()
				s.touch()
				switch action {
				case "remote":
					if _, err := h.call(remote.Request{Command: remote.CommandSave, File: "existing"}); err == nil {
						t.Error("remote save accepted an occupied path")
					}
				case "dialog":
					h.app.saveRequest()
					h.app.save.name.SetText("Existing")
					h.app.save.folder.SetText("")
					h.app.submitSave()
					if h.app.save.err.Peek() == "" {
						t.Error("save dialog did not report the collision")
					}
				case "duplicate":
					req.File = "existing.posting.yaml"
					dup, ok := h.app.saveCopy(req)
					if !ok || dup.File != "existing-copy-2.posting.yaml" {
						t.Fatalf("duplicate = %q, success = %v", dup.File, ok)
					}
				}
				got, err := os.ReadFile(target)
				if err != nil || string(got) != string(original) {
					t.Fatalf("original file changed: %q, error %v", got, err)
				}
				if action != "duplicate" && (s.file.Peek() != "" || !s.dirty.Peek() || h.app.fileExists(file)) {
					t.Fatal("failed creation changed the tab or collection")
				}
			})
		}
	}
}
