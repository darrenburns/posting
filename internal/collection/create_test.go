package collection

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestConcurrentCreatePublishesOneCompleteRequest(t *testing.T) {
	root := t.TempDir()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"First", "Second"} {
		go func() {
			req := model.NewRequest()
			req.File, req.Name = "nested/request.posting.yaml", name
			<-start
			results <- (Dir{Root: root}).Create(req)
		}()
	}
	close(start)
	wins, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			wins++
		case errors.Is(err, fs.ErrExist):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", wins, conflicts)
	}
	loaded, problems := (Dir{Root: root}).Load()
	if len(problems) != 0 || len(loaded.Children) != 1 || len(loaded.Children[0].Requests) != 1 {
		t.Fatalf("loaded=%+v errors=%v", loaded, problems)
	}
	req := loaded.Children[0].Requests[0]
	if req.Name != "First" && req.Name != "Second" {
		t.Fatalf("partial request: %+v", req)
	}
	req.Name = "Updated"
	if err := (Dir{Root: root}).Save(req); err != nil {
		t.Fatal(err)
	}
	loaded, problems = (Dir{Root: root}).Load()
	if len(problems) != 0 || loaded.Children[0].Requests[0].Name != "Updated" {
		t.Fatalf("in-place save failed: %+v, %v", loaded, problems)
	}
	entries, err := os.ReadDir(filepath.Join(root, "nested"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v, %v", entries, err)
	}
}

func TestCreateRejectsOccupiedPaths(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "request.posting.yaml")
			original := []byte("unparseable original bytes")
			var err error
			switch kind {
			case "file":
				err = os.WriteFile(target, original, 0o600)
			case "directory":
				err = os.Mkdir(target, 0o700)
			case "symlink":
				if err := os.WriteFile(filepath.Join(root, "source"), original, 0o600); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink("source", target)
			case "dangling symlink":
				err = os.Symlink("missing", target)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			req := model.NewRequest()
			req.File = "request.posting.yaml"
			if err := (Dir{Root: root}).Create(req); !errors.Is(err, fs.ErrExist) {
				t.Fatalf("create error=%v, want existing-path conflict", err)
			}
			after, err := os.Lstat(target)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("occupied path replaced: %v", err)
			}
			if kind == "file" || kind == "symlink" {
				data, err := os.ReadFile(target)
				if err != nil || string(data) != string(original) {
					t.Fatalf("bytes changed: %q, %v", data, err)
				}
			}
		})
	}
}
