package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
	t "github.com/darrenburns/terma"
)

func uploadApp(tt *testing.T) *App {
	tt.Helper()
	root := tt.TempDir()
	if err := os.Mkdir(filepath.Join(root, "fixtures"), 0700); err != nil {
		tt.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixtures", "report.pdf"), []byte("%PDF-1.7\nExample report\n"), 0600); err != nil {
		tt.Fatal(err)
	}
	a := testApp()
	a.store = collection.Dir{Root: root}
	r := model.NewRequest()
	r.Name = "Upload report"
	r.URL = "https://api.example.com/documents"
	r.Method = model.MethodPost
	r.Body = model.Body{Type: model.BodyMultipart, Multipart: []model.Part{
		{Type: model.PartText, Name: "title", Value: "Quarterly report", Enabled: true},
		{Type: model.PartFile, Name: "document", File: "fixtures/report.pdf", Enabled: true},
		{Type: model.PartText, Name: "metadata", Value: "{\n  \"team\": \"finance\"\n}", ContentType: "application/json", Enabled: true},
		{Type: model.PartFile, Name: "attachment", File: "fixtures/missing.pdf", Enabled: false},
	}}
	a.openRequest(r)
	a.current().requestTab.Set("body")
	return a
}

func TestMultipartInspector(tt *testing.T) {
	a := uploadApp(tt)
	s := a.current()
	p := t.NewPilot(tt, a, 150, 46)
	focus := func(id string) { t.RequestFocus(id); p.Advance(0) }
	focus(partsID)
	p.Press("home", "down")
	p.WaitUntil(func() bool {
		return strings.Contains(p.ScreenText(), "application/pdf") && !strings.Contains(p.ScreenText(), "Checking file")
	}, time.Second)
	if !strings.Contains(p.ScreenText(), "Filename") {
		tt.Fatal("selection did not show file inspector\n" + p.ScreenText())
	}
	p.AssertSnapshot("Multipart_file", "Selected file part, overrides, metadata and a disabled attachment")
	// The parts/inspector divider resizes both panes without changing selection.
	list, _ := p.Bounds(partsID)
	path, _ := p.Bounds("req-part-path")
	x, y := list.X+list.Width, list.Y+1
	p.MouseDown(x, y, uv.MouseLeft, 0)
	p.MouseMove(x+6, y)
	p.MouseUp(x+6, y, uv.MouseLeft, 0)
	resizedList, _ := p.Bounds(partsID)
	resizedPath, _ := p.Bounds("req-part-path")
	if resizedList.Width != list.Width+6 || resizedPath.X != path.X+6 || resizedPath.Width != path.Width-6 {
		tt.Fatalf("divider did not resize the panes: list %+v → %+v, path %+v → %+v", list, resizedList, path, resizedPath)
	}
	p.MouseDown(x+6, y, uv.MouseLeft, 0)
	p.MouseMove(x, y)
	p.MouseUp(x, y, uv.MouseLeft, 0)
	p.Press("ctrl+space")
	if s.Snapshot().Body.Multipart[1].Enabled {
		tt.Fatal("toggle did not disable file")
	}
	p.Press("ctrl+space", "alt+down")
	if got := s.Snapshot().Body.Multipart; got[2].Name != "document" || got[1].Name != "metadata" {
		tt.Fatal("reordering changed parts", got)
	}
	p.Press("up")
	p.AssertSnapshot("Multipart_text", "A JSON text part is edited beside the ordered part list")
	// Switching type and body mode must retain the user's drafts.
	focus("req-part-type")
	p.Press("right", "left")
	if got := s.Snapshot().Body.Multipart[1]; got.Value != "{\n  \"team\": \"finance\"\n}" || got.ContentType != "application/json" {
		tt.Fatal("type switch lost text", got)
	}
	before := s.Snapshot().Body.Multipart
	focus("req-body-type")
	p.Press("right")
	if s.Snapshot().Body.Type != model.BodyFile {
		tt.Fatal("File mode not selected")
	}
	p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "Choose a file") }, time.Second)
	p.AssertSnapshot("Upload_file_body", "A whole-file body has a path and inferred content type")
	p.Press("left")
	if !reflect.DeepEqual(s.Snapshot().Body.Multipart, before) {
		tt.Fatal("body mode switch lost parts")
	}
	focus(partsID)
	p.Press("end", "ctrl+space", "enter")
	p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "File not found") }, time.Second)
	p.AssertSnapshot("Multipart_missing", "Missing file is visible both in the list and inspector")
	focus("req-part-path")
	p.Press("ctrl+a")
	p.Type("fixtures/rep")
	p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "fixtures/report.pdf") }, time.Second)
	p.Press("enter")
	if got := s.Snapshot().Body.Multipart[3].File; got != "fixtures/report.pdf" {
		tt.Fatalf("completed path = %q", got)
	}
	p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "24 B · application/pdf") }, time.Second)
	// At narrower widths, Enter opens details and Escape returns to parts.
	p.Resize(100, 40)
	focus("req-part-name")
	p.Press("escape")
	focus(partsID)
	p.Press("home", "enter")
	if !strings.Contains(p.ScreenText(), "‹ Parts") || !strings.Contains(p.ScreenText(), "Quarterly report") {
		tt.Fatal("narrow inspector missing\n" + p.ScreenText())
	}
	p.AssertSnapshot("Multipart_narrow", "The selected part opens full-width on a narrow terminal")
	p.Press("escape")
	if p.FocusedID() != partsID {
		tt.Fatalf("Escape focused %q", p.FocusedID())
	}
	p.Press("ctrl+x")
	if len(s.Snapshot().Body.Multipart) != 3 {
		tt.Fatal("Delete did not remove a part")
	}
	p.Press("insert")
	p.Type("extra")
	focus("req-part-text")
	p.Type("A new text part")
	if got := s.Snapshot().Body.Multipart; len(got) != 4 || got[3].Name != "extra" || got[3].Value != "A new text part" || !got[3].Enabled {
		tt.Fatal("new part was not editable", got)
	}
}

func TestUploadPathCompletion(tt *testing.T) {
	for _, mode := range []model.BodyType{model.BodyMultipart, model.BodyFile} {
		tt.Run(string(mode), func(tt *testing.T) {
			a := uploadApp(tt)
			if err := os.WriteFile(filepath.Join(a.uploadRoot(), "fixtures", "annual report.pdf"), []byte("%PDF-1.7\nAnnual report\n"), 0600); err != nil {
				tt.Fatal(err)
			}
			s := a.current()
			s.bodyType.Set(mode)
			p := t.NewPilot(tt, a, 150, 46)
			id := "req-file-path"
			path := func() string { return s.Snapshot().Body.File }
			if mode == model.BodyMultipart {
				t.RequestFocus(partsID)
				p.Advance(0)
				p.Press("home", "down")
				id = "req-part-path"
				path = func() string { return s.Snapshot().Body.Multipart[1].File }
			}
			t.RequestFocus(id)
			p.Advance(0)
			p.Press("ctrl+a")
			p.Type("fi")
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "fixtures/") }, time.Second)
			p.Press("enter")
			if got := path(); got != "fixtures/" {
				tt.Fatalf("directory completion = %q", got)
			}
			p.Type("annual")
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "fixtures/annual report.pdf") }, time.Second)
			p.Press("enter")
			if got := path(); got != "fixtures/annual report.pdf" {
				tt.Fatalf("file completion = %q", got)
			}
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "23 B · application/pdf") }, time.Second)
			// Completing a variable must still allow file completion beneath it.
			a.sessionVars.Set(map[string]string{"FIXTURES": "fixtures"})
			p.Press("ctrl+a")
			p.Type("${FIX")
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "${FIXTURES}") }, time.Second)
			p.Press("enter")
			if got := path(); got != "${FIXTURES}" {
				tt.Fatalf("variable completion = %q", got)
			}
			p.Type("/annual")
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "${FIXTURES}/annual report.pdf") }, time.Second)
			p.Press("enter")
			if got := path(); got != "${FIXTURES}/annual report.pdf" {
				tt.Fatalf("variable-rooted file completion = %q", got)
			}
			p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "23 B · application/pdf") }, time.Second)
		})
	}
}

func TestUploadEscapeCancels(tt *testing.T) {
	for _, mode := range []model.BodyType{model.BodyMultipart, model.BodyFile} {
		tt.Run(string(mode), func(tt *testing.T) {
			a := uploadApp(tt)
			s := a.current()
			s.bodyType.Set(mode)
			s.upload.text.SetText("fixtures/report.pdf")
			if err := os.Truncate(filepath.Join(a.uploadRoot(), "fixtures", "report.pdf"), 16<<20); err != nil {
				tt.Fatal(err)
			}
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-release // Keep the upload in flight instead of buffering the entire body.
				io.Copy(io.Discard, r.Body)
			}))
			defer server.Close()
			defer close(release)
			h := client.NewHTTP("", client.TLSSettings{})
			h.Root = a.uploadRoot()
			a.sender = h
			s.url.SetText(server.URL)
			p := t.NewPilot(tt, a, 150, 46)
			focusID := "req-file-content-type"
			if mode == model.BodyMultipart {
				t.RequestFocus(partsID)
				p.Advance(0)
				p.Press("home", "down", "enter")
				focusID = "req-part-name"
			}
			t.RequestFocus(focusID)
			p.Advance(0)
			p.Press("ctrl+j")
			p.WaitUntil(func() bool {
				progress := s.uploadProgress.Peek()
				return progress.sent > 0 && progress.sent < progress.total && strings.Contains(p.ScreenText(), "Uploading")
			}, 3*time.Second)
			p.Press("escape")
			p.WaitUntil(func() bool { return s.phase.Peek() == exchangeCancelled }, 3*time.Second)
			if !strings.Contains(p.ScreenText(), "Request cancelled") {
				tt.Fatal("Escape did not report cancellation\n" + p.ScreenText())
			}
		})
	}
}
