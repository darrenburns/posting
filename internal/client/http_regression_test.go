package client

import (
	"compress/flate"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

func TestHTTPCustomHostHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.Host)
	}))
	defer server.Close()
	for _, name := range []string{"Host", "host", "HOST"} {
		t.Run(name, func(t *testing.T) {
			req := model.NewRequest()
			req.URL = server.URL
			req.Headers = []model.KeyValue{{Name: name, Value: "virtual.example", Enabled: true}}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if string(resp.Body) != "virtual.example" {
				t.Errorf("server received Host %q, want virtual.example", resp.Body)
			}
		})
	}
}

func TestHTTPDeflateResponse(t *testing.T) {
	const payload = "a readable deflate response"
	for _, raw := range []bool{false, true} {
		name := "zlib"
		if raw {
			name = "raw"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", "deflate")
				var compressed io.WriteCloser
				if raw {
					compressed, _ = flate.NewWriter(w, flate.DefaultCompression)
				} else {
					compressed = zlib.NewWriter(w)
				}
				io.WriteString(compressed, payload)
				compressed.Close()
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL = server.URL
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if string(resp.Body) != payload {
				t.Errorf("response was not decoded: got %q, want %q", resp.Body, payload)
			}
		})
	}
}
