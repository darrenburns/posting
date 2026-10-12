package collection

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestUploadRequestRoundTrip(t *testing.T) {
	for _, body := range []string{
		"file: missing/fixture.bin\ncontent_type: application/octet-stream\n",
		"file: ''\n",
		"multipart: []\n",
		`multipart:
  - name: tag
    value: ''
  - name: file
    file: '${ROOT}/not-present.pdf'
    filename: 'report.pdf'
    content_type: application/pdf
  - name: tag
    value: '{"team":"finance"}'
    content_type: application/json
    enabled: false
`,
	} {
		t.Run(body, func(t *testing.T) {
			data := "method: POST\nurl: https://example.test\nbody:\n  " + strings.ReplaceAll(strings.TrimSuffix(body, "\n"), "\n", "\n  ") + "\n"
			req, err := ParseRequest([]byte(data), "nested/upload.posting.yaml")
			if err != nil {
				t.Fatal(err)
			}
			out, err := MarshalRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			back, err := ParseRequest(out, req.File)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(req, back) {
				t.Fatalf("save/reload changed upload: %+v / %+v\n%s", req.Body, back.Body, out)
			}
			encoded, _ := json.Marshal(req)
			var history model.Request
			if err := json.Unmarshal(encoded, &history); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(req, history) {
				t.Fatal("history changed file references")
			}
			resolved, err := model.Resolve(req, model.MapLookup(map[string]string{"ROOT": "fixtures"}))
			if err != nil {
				t.Fatal(err)
			}
			if len(req.Body.Multipart) > 1 && resolved.Body.Multipart[1].File != "fixtures/not-present.pdf" {
				t.Fatal(resolved.Body)
			}
			if len(req.Body.Multipart) > 1 && req.Body.Multipart[1].File != "${ROOT}/not-present.pdf" {
				t.Fatal("resolution mutated original")
			}
			req.Options.SubstituteBodyVariables = false
			literal, _ := model.Resolve(req, model.MapLookup(map[string]string{"ROOT": "fixtures"}))
			if !reflect.DeepEqual(req.Body, literal.Body) {
				t.Fatal("literal body changed")
			}
		})
	}
	for _, body := range []string{"{file: x, content: y}", "{multipart: [{name: a, file: x, value: y}]}", "{multipart: [{name: a}]}"} {
		if _, err := ParseRequest([]byte("body: "+body), ""); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
}
