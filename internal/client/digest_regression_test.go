package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
)

// Independently decode quoted strings so a matching bug in our challenge parser
// cannot hide an incorrectly encoded Authorization header.
func digestCredentials(header string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(header, "Digest "), ", ") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strings.HasPrefix(value, `"`) {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				continue
			}
		}
		values[key] = value
	}
	return values
}

func validDigest(r *http.Request, username, password, realm, nonce string) bool {
	p := digestCredentials(r.Header.Get("Authorization"))
	hash := func(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }
	ha1 := hash(username + ":" + realm + ":" + password)
	ha2 := hash(r.Method + ":" + r.URL.RequestURI())
	expected := hash(strings.Join([]string{ha1, nonce, p["nc"], p["cnonce"], "auth", ha2}, ":"))
	return p["username"] == username && p["realm"] == realm && p["nonce"] == nonce && p["uri"] == r.URL.RequestURI() && p["response"] == expected
}

func TestHTTPDigestAfterRedirect(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		method               model.Method
		status               int
		wantMethod, wantBody string
	}{
		{"GET_302", model.MethodGet, 302, "GET", ""},
		{"POST_301", model.MethodPost, 301, "GET", ""},
		{"POST_302", model.MethodPost, 302, "GET", ""},
		{"POST_303", model.MethodPost, 303, "GET", ""},
		{"POST_307", model.MethodPost, 307, "POST", "payload"},
		{"POST_308", model.MethodPost, 308, "POST", "payload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts, challenges := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/start" {
					starts++
					if r.Header.Get("Authorization") != "" {
						t.Error("sent destination credentials to redirecting endpoint")
					}
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "anonymous", Path: "/"})
					http.Redirect(w, r, "/private?x=1", tc.status)
					return
				}
				body, _ := io.ReadAll(r.Body)
				if r.Method != tc.wantMethod || string(body) != tc.wantBody {
					t.Errorf("destination received %s %q, want %s %q", r.Method, body, tc.wantMethod, tc.wantBody)
				}
				if !validDigest(r, "ada", "secret", "test", "abc123") {
					challenges++
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "authenticated", Path: "/"})
					w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="abc123", qop="auth", algorithm=MD5`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if got := r.Header.Get("Cookie"); got != "preference=light; session=authenticated" {
					t.Errorf("authenticated retry Cookie = %q", got)
				}
				io.WriteString(w, "welcome")
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL, req.Method = server.URL+"/start", tc.method
			if tc.method == model.MethodPost {
				req.Body = model.Body{Type: model.BodyRaw, Raw: "payload"}
			}
			req.Headers = []model.KeyValue{{Name: "Cookie", Value: "preference=light", Enabled: true}}
			req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if resp.StatusCode != http.StatusOK || string(resp.Body) != "welcome" {
				t.Errorf("digest after redirect = %d %q, want 200 welcome", resp.StatusCode, resp.Body)
			}
			if starts != 1 || challenges != 1 {
				t.Errorf("redirect endpoint called %d times, challenges = %d; want 1 each", starts, challenges)
			}
		})
	}
}

func TestHTTPDigestDoesNotAnswerUnrelatedHost(t *testing.T) {
	authSeen := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen = authSeen || r.Header.Get("Authorization") != ""
		w.Header().Set("WWW-Authenticate", `Digest realm="other", nonce="abc", qop="auth"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen = authSeen || r.Header.Get("Authorization") != ""
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer origin.Close()
	req := model.NewRequest()
	req.URL = strings.Replace(origin.URL, "127.0.0.1", "localhost", 1)
	req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
	resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
	if resp.StatusCode != http.StatusUnauthorized || authSeen {
		t.Errorf("cross-host challenge: status=%d credentials sent=%v", resp.StatusCode, authSeen)
	}
}

func TestHTTPDigestQuotedCredentials(t *testing.T) {
	for _, tc := range []struct{ name, username, realm, nonce string }{
		{"plain", "ada", "test", "abc"},
		{"domain_username", `DOMAIN\ada`, "test", "abc"},
		{"escaped_challenge", "ada", `area\private`, `nonce\token`},
		{"backslash_before_quote", `ada\"name`, `area\"private`, `nonce\"token`},
		{"trailing_backslash", `ada\`, `area\`, `nonce\`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !validDigest(r, tc.username, "secret", tc.realm, tc.nonce) {
					w.Header().Set("WWW-Authenticate", fmt.Sprintf("Digest realm=%s, nonce=%s, qop=\"auth\"", strconv.Quote(tc.realm), strconv.Quote(tc.nonce)))
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				io.WriteString(w, "welcome")
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL = server.URL
			req.Auth = model.Auth{Type: model.AuthDigest, Username: tc.username, Password: "secret"}
			resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("valid credentials rejected: status %d", resp.StatusCode)
			}
		})
	}
}

func TestHTTPDigestRedirectPolicy(t *testing.T) {
	for _, follow := range []bool{false, true} {
		t.Run(fmt.Sprintf("follow_%t", follow), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.Redirect(w, r, "/loop", http.StatusFound)
			}))
			defer server.Close()
			req := model.NewRequest()
			req.URL = server.URL
			req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
			req.Options.FollowRedirects = follow
			resp, err := NewHTTP("", TLSSettings{}).Send(context.Background(), Call{Request: req})
			if follow {
				if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") || calls != 10 {
					t.Errorf("redirect limit: response=%v err=%v calls=%d", resp, err, calls)
				}
			} else if err != nil || resp.StatusCode != http.StatusFound || calls != 1 {
				t.Errorf("redirects disabled: response=%v err=%v calls=%d", resp, err, calls)
			}
		})
	}
}

func TestHTTPDigestRetryWithCookiesDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "explicit=value" {
			t.Errorf("Cookie = %q, want explicit=value", got)
		}
		if !validDigest(r, "ada", "secret", "test", "abc") {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "from-server"})
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="abc", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, "welcome")
	}))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	req.Options.AttachCookies = false
	req.Headers = []model.KeyValue{{Name: "Cookie", Value: "explicit=value", Enabled: true}}
	req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
	resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("retry with cookies disabled = %d", resp.StatusCode)
	}
}
