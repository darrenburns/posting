package themes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPosting2Themes(t *testing.T) {
	got, problems := LoadDir("../../tests/sample-themes")
	if len(problems) > 0 {
		t.Fatalf("problems: %v", problems)
	}
	if len(got) == 0 {
		t.Fatal("no themes loaded")
	}
	var ocean *Theme
	for i := range got {
		if got[i].Name == "serene_ocean" {
			ocean = &got[i]
		}
	}
	if ocean == nil || ocean.Primary != "#1E88E5" || ocean.IsDark() || ocean.Text != "#212121" {
		t.Fatalf("serene_ocean = %+v", ocean)
	}
}

func TestInvalidThemesAreReported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ok.yaml"), []byte("name: ok\nprimary: '#abc'\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "noname.yaml"), []byte("primary: '#abcdef'\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "badcolour.yml"), []byte("name: bad\nprimary: blue\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o644)
	got, problems := LoadDir(dir)
	if len(got) != 1 || got[0].Name != "ok" || len(problems) != 2 {
		t.Fatalf("themes = %+v, problems = %v", got, problems)
	}
	if got, problems := LoadDir(filepath.Join(dir, "missing")); got != nil || problems != nil {
		t.Fatal("a missing directory has no themes and no problems")
	}
}
