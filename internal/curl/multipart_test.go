package curl

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

func TestUploadCurlWire(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl required")
	}
	type part struct {
		Name, Filename, ContentType string
		Data                        []byte
	}
	root := t.TempDir()
	filename := `report,"one";.pdf`
	data := []byte("\x00\xff${LITERAL}\n")
	if err := os.WriteFile(filepath.Join(root, filename), data, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var parts []part
		reader, err := r.MultipartReader()
		if err != nil {
			b, _ := io.ReadAll(r.Body)
			parts = append(parts, part{ContentType: r.Header.Get("Content-Type"), Data: b})
		} else {
			for {
				p, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Error(err)
					return
				}
				b, _ := io.ReadAll(p)
				parts = append(parts, part{p.FormName(), p.FileName(), p.Header.Get("Content-Type"), b})
			}
		}
		json.NewEncoder(w).Encode(parts)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name string
		body model.Body
		want []part
	}{
		{"empty", model.Body{Type: model.BodyMultipart}, nil},
		{"disabled", model.Body{Type: model.BodyMultipart, Multipart: []model.Part{{Type: model.PartFile, Name: "disabled", File: "missing"}}}, nil},
		{"multipart", model.Body{Type: model.BodyMultipart, Multipart: []model.Part{
			{Type: model.PartText, Name: "literal", Value: "@not-a-file;type=fake", Enabled: true},
			{Type: model.PartFile, Name: "document", File: filename, Filename: `résumé "one".pdf`, ContentType: "application/pdf", Enabled: true},
			{Type: model.PartText, Name: "literal", Value: "  @literal; with, comma  ", ContentType: "text/plain", Enabled: true},
			{Type: model.PartFile, Name: "disabled", File: "missing"},
			{Type: model.PartText, Name: "metadata", Value: `{"team":"finance"}`, ContentType: "application/json", Enabled: true},
		}}, []part{
			{"literal", "", "", []byte("@not-a-file;type=fake")},
			{"document", `résumé "one".pdf`, "application/pdf", data},
			{"literal", "", "text/plain", []byte("  @literal; with, comma  ")},
			{"metadata", "", "application/json", []byte(`{"team":"finance"}`)},
		}},
		{"file", model.Body{Type: model.BodyFile, File: filename, ContentType: "application/pdf"}, []part{{ContentType: "application/pdf", Data: data}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := model.NewRequest()
			req.URL = server.URL
			req.Method = model.MethodPost
			req.Body = tc.body
			want := tc.want
			command := Format(req, FormatOptions{Root: root})
			args, err := Split(command)
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("curl", append([]string{"-q", "-sS", "--noproxy", "*", "--max-time", "5"}, args[1:]...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s\n%s", err, command, out)
			}
			var actual []part
			if err := json.Unmarshal(out, &actual); err != nil {
				t.Fatalf("%v: %s\n%s", err, out, command)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("curl sent %#v, want %#v\n%s", actual, want, command)
			}
			parsed, err := Parse(command)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.NewHTTP("", client.TLSSettings{}).Send(context.Background(), client.Call{Request: parsed})
			if err != nil {
				t.Fatal(err)
			}
			var imported []part
			if err := json.Unmarshal(resp.Body, &imported); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(imported, want) {
				t.Fatalf("Posting sent %#v, want %#v", imported, want)
			}
		})
	}
}

func TestParseUploadReferencesWithoutReading(t *testing.T) {
	req, err := Parse(`curl -F 'file=@missing.pdf;filename=sent.pdf;type=application/pdf' -F 'tag=one,two' --form-string 'tag=<missing;type=json' https://example.test`)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Body.Multipart; len(got) != 3 || got[0].File != "missing.pdf" || got[0].Filename != "sent.pdf" || got[1].Value != "one,two" || got[2].Value != "<missing;type=json" {
		t.Fatal(got)
	}
	req, err = Parse(`curl --data-binary @missing.bin https://example.test`)
	if err != nil || req.Body.Type != model.BodyFile || req.Body.File != "missing.bin" || req.Body.ContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("%+v %v", req.Body, err)
	}
	for _, flags := range []string{`-F 'x=@-'`, `-F 'x=<file'`, `-F 'x=@one,two'`, `-F 'x=y;headers=@secret'`, `-F 'x=y' -d 'z=q'`, `--data-binary @file -d text`} {
		if _, err := Parse("curl " + flags + " https://example.test"); err == nil {
			t.Errorf("accepted %s", flags)
		}
	}
}
