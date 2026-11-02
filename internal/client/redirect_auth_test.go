package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestHTTPRedirectDowngradeDropsAuthorization(t *testing.T) {
	for _, auth := range []string{"basic", "bearer", "header", "digest"} {
		t.Run(auth, func(t *testing.T) {
			var origin *httptest.Server
			var visited []string
			plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				visited = append(visited, r.URL.Path)
				if got := r.Header.Get("Authorization"); got != "" {
					t.Errorf("plaintext %s received Authorization %q", r.URL.Path, got)
				}
				if r.URL.Path == "/first" {
					http.Redirect(w, r, origin.URL+"/return", http.StatusFound)
				}
			}))
			t.Cleanup(plain.Close)
			origin = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/return" {
					if got := r.Header.Get("Authorization"); got != "" {
						t.Errorf("redirect after downgrade restored Authorization %q", got)
					}
					http.Redirect(w, r, plain.URL+"/last", http.StatusFound)
					return
				}
				if auth == "digest" && !validDigest(r, "ada", "secret", "test", "abc") {
					w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="abc", qop="auth"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Header.Get("Authorization") == "" {
					t.Error("origin did not receive authorization")
				}
				if r.URL.Path == "/" {
					http.Redirect(w, r, "/secure", http.StatusFound)
					return
				}
				http.Redirect(w, r, plain.URL+"/first", http.StatusFound)
			}))
			t.Cleanup(origin.Close)
			req := model.NewRequest()
			req.URL = origin.URL
			req.Options.VerifySSL = false
			switch auth {
			case "basic":
				req.Auth = model.Auth{Type: model.AuthBasic, Username: "ada", Password: "secret"}
			case "digest":
				req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
			case "bearer":
				req.Auth = model.Auth{Type: model.AuthBearer, Token: "synthetic-token"}
			case "header":
				req.Headers = []model.KeyValue{{Name: "Authorization", Value: "Custom synthetic-token", Enabled: true}}
			}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if resp.StatusCode != http.StatusOK || len(visited) != 2 || visited[0] != "/first" || visited[1] != "/last" {
				t.Fatalf("status %d, plaintext visits %v", resp.StatusCode, visited)
			}
		})
	}
}

func TestHTTPDigestRedirectAfterAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, wantMethod, wantBody string
		method                     model.Method
		status                     int
	}{
		{"GET_302", "GET", "", model.MethodGet, 302},
		{"POST_303", "GET", "", model.MethodPost, 303},
		{"POST_307", "POST", "payload", model.MethodPost, 307},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !validDigest(r, "ada", "secret", "test", "abc") {
					w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="abc", qop="auth"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/final?x=1", tc.status)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != tc.wantMethod || string(body) != tc.wantBody {
					t.Errorf("destination received %s %q, error %v", r.Method, body, err)
				}
				io.WriteString(w, "welcome")
			}))
			t.Cleanup(server.Close)
			req := model.NewRequest()
			req.URL, req.Method = server.URL+"/start", tc.method
			req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
			if tc.method == model.MethodPost {
				req.Body = model.Body{Type: model.BodyRaw, Raw: "payload"}
			}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if resp.StatusCode != http.StatusOK || string(resp.Body) != "welcome" {
				t.Fatalf("response %d %q, want 200 welcome", resp.StatusCode, resp.Body)
			}
		})
	}
}
