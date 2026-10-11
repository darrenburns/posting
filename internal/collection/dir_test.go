package collection

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestDirRejectsOutsideParentSymlink(t *testing.T) {
	for _, relative := range []bool{false, true} {
		t.Run(map[bool]string{false: "absolute", true: "relative"}[relative], func(t *testing.T) {
			base := t.TempDir()
			dir := Dir{Root: filepath.Join(base, "collection")}
			outside := filepath.Join(base, "outside")
			for _, folder := range []string{dir.Root, outside} {
				if err := os.Mkdir(folder, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			target := outside
			if relative {
				target = "../outside"
			}
			if err := os.Symlink(target, filepath.Join(dir.Root, "escape")); err != nil {
				t.Fatal(err)
			}
			const original = "name: outside\n"
			victim := filepath.Join(outside, "existing.posting.yaml")
			if err := os.WriteFile(victim, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{"escape/new.posting.yaml", "escape/nested/new.posting.yaml", "escape/existing.posting.yaml"} {
				req := model.NewRequest()
				req.File = file
				if err := dir.Save(req); err == nil {
					t.Errorf("Save(%q) succeeded through an outside parent symlink", file)
				}
			}
			if err := dir.Delete("escape/existing.posting.yaml"); err == nil {
				t.Error("Delete succeeded through an outside parent symlink")
			}
			data, err := os.ReadFile(victim)
			if err != nil || string(data) != original {
				t.Errorf("outside request changed: data = %q, error = %v", data, err)
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "existing.posting.yaml" {
				t.Errorf("save or delete changed the outside directory: %v", entries)
			}
		})
	}
}

func TestDirSaveAndDeleteThroughInternalSymlink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "collection")
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	rootLink := filepath.Join(base, "collection-link")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Fatal(err)
	}
	dir := Dir{Root: rootLink}
	req := model.NewRequest()
	req.File = "alias/nested/new.posting.yaml"
	req.Name = "Saved through a link"
	if err := dir.Save(req); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "real/nested/new.posting.yaml")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseRequest(data, req.File)
	if err != nil || got.Name != req.Name {
		t.Fatalf("saved request = %+v, error = %v", got, err)
	}
	if err := dir.Delete(req.File); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("request still exists after delete: %v", err)
	}
}

func TestDirSaveAndDeleteLeafSymlink(t *testing.T) {
	for _, operation := range []string{"save", "delete"} {
		t.Run(operation, func(t *testing.T) {
			dir := Dir{Root: t.TempDir()}
			outside := filepath.Join(t.TempDir(), "outside.posting.yaml")
			const original = "name: outside\n"
			if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			req := model.NewRequest()
			req.File = "leaf.posting.yaml"
			req.Name = "Replacement"
			leaf := filepath.Join(dir.Root, req.File)
			if err := os.Symlink(outside, leaf); err != nil {
				t.Fatal(err)
			}
			if operation == "save" {
				if err := dir.Save(req); err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(leaf)
				if err != nil || !info.Mode().IsRegular() {
					t.Fatalf("save did not replace the symlink with a regular file: %v", err)
				}
				data, err := os.ReadFile(leaf)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ParseRequest(data, req.File)
				if err != nil || got.Name != req.Name {
					t.Fatalf("replacement request = %+v, error = %v", got, err)
				}
			} else {
				if err := dir.Delete(req.File); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(leaf); !os.IsNotExist(err) {
					t.Fatalf("leaf still exists after delete: %v", err)
				}
			}
			data, err := os.ReadFile(outside)
			if err != nil || string(data) != original {
				t.Fatalf("symlink target changed: data = %q, error = %v", data, err)
			}
		})
	}
}

func TestDirMissingPaths(t *testing.T) {
	dir := Dir{Root: filepath.Join(t.TempDir(), "missing", "collection")}
	req := model.NewRequest()
	req.File = "nested/new.posting.yaml"
	if err := dir.Delete(req.File); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir.Root); !os.IsNotExist(err) {
		t.Fatalf("delete created the missing collection: %v", err)
	}
	if err := dir.Save(req); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{req.File, req.File, "absent/new.posting.yaml"} {
		if err := dir.Delete(file); err != nil {
			t.Fatalf("Delete(%q): %v", file, err)
		}
	}
}
