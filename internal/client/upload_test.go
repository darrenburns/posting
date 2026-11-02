package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestHTTPUploadsPartsInOrder(t *testing.T) {
	root := t.TempDir()
	data := []byte("\x00\xff${SECRET}\r\nfile bytes")
	if err := os.WriteFile(filepath.Join(root, "report.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	type part struct {
		Name, Filename, Type string
		Bytes                []byte
	}
	want := []part{{"tag", "", "", []byte("first")}, {"document", "résumé.pdf", "application/pdf", data}, {"tag", "", "application/json", []byte(`{"n":3}`)}, {"empty", "empty", "application/octet-stream", []byte{}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength <= 0 {
			t.Error("upload must have a known Content-Length")
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var got []part
		for {
			p, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			data, err := io.ReadAll(p)
			if err != nil {
				t.Error(err)
				return
			}
			got = append(got, part{p.FormName(), p.FileName(), p.Header.Get("Content-Type"), data})
		}
		json.NewEncoder(w).Encode(got)
	}))
	defer server.Close()
	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = server.URL
	req.Headers = []model.KeyValue{{Name: "Content-Type", Value: "multipart/form-data", Enabled: true}}
	req.Body = model.Body{Type: model.BodyMultipart, Multipart: []model.Part{
		{Type: model.PartText, Name: "tag", Value: "${TAG}", Enabled: true},
		{Type: model.PartFile, Name: "document", File: "${FILE}", Filename: "résumé.pdf", Enabled: true},
		{Type: model.PartFile, Name: "missing", File: "does-not-exist", Enabled: false},
		{Type: model.PartText, Name: "tag", Value: `{"n":3}`, ContentType: "application/json", Enabled: true},
		{Type: model.PartFile, Name: "empty", File: "empty", Enabled: true},
	}}
	h := NewHTTP("", TLSSettings{})
	h.Root = root
	resp := send(t, h, req, map[string]string{"TAG": "first", "FILE": "report.pdf", "SECRET": "must not replace"})
	var got []part
	if err := json.Unmarshal(resp.Body, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %#v, want %#v", got, want)
	}
}

func TestHTTPUploadReplayAndCurrentFile(t *testing.T) {
	for _, status := range []int{307, 308, 401} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "file.bin")
			os.WriteFile(path, []byte("original\x00${X}"), 0600)
			var bodies [][]byte
			var types []string
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				mu.Lock()
				bodies = append(bodies, body)
				types = append(types, r.Header.Get("Content-Type"))
				n := len(bodies)
				mu.Unlock()
				if n == 1 {
					w.Header().Set("Location", "/again")
					w.Header().Set("WWW-Authenticate", `Digest realm="upload", nonce="nonce", qop="auth"`)
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL = server.URL
			req.Method = model.MethodPost
			req.Auth = model.Auth{Type: model.AuthDigest, Username: "u", Password: "p"}
			req.Body = model.Body{Type: model.BodyMultipart, Multipart: []model.Part{{Type: model.PartFile, Name: "file", File: "file.bin", Enabled: true}}}
			h := NewHTTP("", TLSSettings{})
			h.Root = root
			if resp := send(t, h, req, nil); resp.StatusCode != 200 {
				t.Fatal(resp.StatusCode)
			}
			mu.Lock()
			if len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) || types[0] != types[1] {
				t.Errorf("replay changed body or boundary: %d sends", len(bodies))
			}
			mu.Unlock()
			os.WriteFile(path, []byte("new contents"), 0600)
			req.Body = model.Body{Type: model.BodyFile, File: "file.bin", ContentType: "application/x-test"}
			send(t, h, req, nil)
			mu.Lock()
			defer mu.Unlock()
			if string(bodies[2]) != "new contents" || types[2] != "application/x-test" {
				t.Fatal("re-send must read current whole file")
			}
		})
	}
}

func TestHTTPUploadRejectsBeforeConnecting(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ok"), nil, 0600)
	for _, tc := range []struct {
		body   model.Body
		header string
	}{
		{model.Body{Type: model.BodyFile, File: "missing"}, ""},
		{model.Body{Type: model.BodyFile, File: "."}, ""},
		{model.Body{Type: model.BodyFile}, ""},
		{model.Body{Type: model.BodyMultipart, Multipart: []model.Part{{Type: model.PartFile, Name: "f", File: "missing", Enabled: true}}}, ""},
		{model.Body{Type: model.BodyMultipart, Multipart: []model.Part{{Type: model.PartText, Name: "bad\r\nHeader", Enabled: true}}}, ""},
		{model.Body{Type: model.BodyMultipart}, "application/json"},
		{model.Body{Type: model.BodyMultipart}, "multipart/form-data; boundary=stale"},
	} {
		req := model.NewRequest()
		req.Method = model.MethodPost
		req.URL = server.URL
		req.Body = tc.body
		if tc.header != "" {
			req.Headers = []model.KeyValue{{Name: "Content-Type", Value: tc.header, Enabled: true}}
		}
		h := NewHTTP("", TLSSettings{})
		h.Root = root
		if _, err := h.Send(context.Background(), Call{Request: req}); err == nil {
			t.Errorf("accepted invalid body %+v, header %q", tc.body, tc.header)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid upload connected %d times", calls.Load())
	}
}

func TestHTTPUploadChangedDuringRedirect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	os.WriteFile(path, []byte("before"), 0600)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.Copy(io.Discard, r.Body)
		os.WriteFile(path, []byte("after and larger"), 0600)
		w.Header().Set("Location", "/again")
		w.WriteHeader(307)
	}))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	req.Method = model.MethodPut
	req.Body = model.Body{Type: model.BodyFile, File: path}
	_, err := NewHTTP("", TLSSettings{}).Send(context.Background(), Call{Request: req})
	if err == nil || !strings.Contains(err.Error(), "changed during this send") || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestHTTPUploadProgressAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(16 << 20)
	f.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body) }))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	req.Method = model.MethodPut
	req.Body = model.Body{Type: model.BodyFile, File: path}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var last, total int64
	var mu sync.Mutex
	start := time.Now()
	_, err = NewHTTP("", TLSSettings{}).Send(ctx, Call{Request: req, OnUpload: func(sent, size int64) {
		mu.Lock()
		last, total = sent, size
		mu.Unlock()
		if sent > 0 {
			cancel()
		}
	}})
	mu.Lock()
	defer mu.Unlock()
	if err == nil || last <= 0 || total != 16<<20 || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel err=%v progress=%d/%d elapsed=%s", err, last, total, time.Since(start))
	}
}
