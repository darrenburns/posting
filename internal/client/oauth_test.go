package client

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
)

// Substitute only the OS vault, not the OAuth exchange or resource server.
type memoryTokens struct {
	mu     sync.Mutex
	values map[string]string
	err    error
}

func (s *memoryTokens) Get(k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	if v, ok := s.values[k]; ok {
		return v, nil
	}
	return "", keyring.ErrNotFound
}
func (s *memoryTokens) Set(k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[k] = v
	return nil
}
func (s *memoryTokens) Delete(k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	delete(s.values, k)
	return nil
}

func oauthHTTP(store tokenStore, open func(string) error) *HTTP {
	h := NewHTTP("test", TLSSettings{})
	h.OAuth = NewOAuth("test-collection", TLSSettings{}, open)
	h.OAuth.store = store
	return h
}

func oauthRequest(base, grant string) model.Request {
	r := model.NewRequest()
	r.URL = base + "/resource"
	r.Auth = model.Auth{Type: model.AuthOAuth2, OAuth2: model.OAuth2{
		GrantType: grant, AuthorizationURL: base + "/authorize", TokenURL: base + "/token",
		ClientID: "client", ClientSecret: "${SECRET}", Scope: "read write", RedirectURI: "http://127.0.0.1:0/oauth/callback",
	}}
	return r
}

func TestOAuthClientCredentialsReuseRenewForgetAndIsolation(t *testing.T) {
	for _, style := range []string{"client_secret_post", "client_secret_basic"} {
		t.Run(style, func(t *testing.T) {
			var grants atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/resource" {
					io.WriteString(w, r.Header.Get("Authorization"))
					return
				}
				r.ParseForm()
				id, secret := r.Form.Get("client_id"), r.Form.Get("client_secret")
				if style == "client_secret_basic" {
					id, secret, _ = r.BasicAuth()
					secret, _ = url.QueryUnescape(secret)
					if r.Form.Has("client_secret") {
						t.Error("Basic auth also sent secret in body")
					}
				}
				if id != "client" || secret != "secret&$literal" || r.Form.Get("grant_type") != "client_credentials" {
					t.Error("incorrect client credential exchange")
				}
				n := grants.Add(1)
				expires := 3600
				if n == 1 {
					expires = 1
				} // Within OAuth's early-expiry window.
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"access_token":"access-%d","token_type":"Bearer","expires_in":%d}`, n, expires)
			}))
			defer server.Close()
			store := &memoryTokens{}
			h := oauthHTTP(store, nil)
			req := oauthRequest(server.URL, "client_credentials")
			req.Auth.OAuth2.ClientAuth = style
			// Switching to client credentials keeps the code grant's fields
			// in the editor, but their unused variables must not block a send.
			req.Auth.OAuth2.AuthorizationURL = "${UNUSED_AUTH_URL}"
			req.Auth.OAuth2.RedirectURI = "${UNUSED_REDIRECT}"
			vars := map[string]string{"SECRET": "secret&$literal"}
			if got := string(send(t, h, req, vars).Body); got != "Bearer access-1" {
				t.Fatal(got)
			}
			// Concurrent callers renew once, then share the renewed token.
			var wg sync.WaitGroup
			for range 6 {
				wg.Go(func() {
					resp, err := h.Send(context.Background(), Call{Request: req, Variables: vars})
					if err != nil {
						t.Error(err)
						return
					}
					if string(resp.Body) != "Bearer access-2" {
						t.Error(string(resp.Body))
					}
				})
			}
			wg.Wait()
			if grants.Load() != 2 {
				t.Fatalf("%d grants, want 2", grants.Load())
			}
			// A new process can reuse the vault, without acquiring again.
			h = oauthHTTP(store, nil)
			if got := string(send(t, h, req, vars).Body); got != "Bearer access-2" {
				t.Fatal(got)
			}
			if err := h.OAuth.Forget(context.Background(), Call{Request: req, Variables: vars}); err != nil {
				t.Fatal(err)
			}
			if got := string(send(t, h, req, vars).Body); got != "Bearer access-3" {
				t.Fatal(got)
			}
			other := req.Clone()
			other.Auth.OAuth2.Scope = "admin"
			if got := string(send(t, h, other, vars).Body); got != "Bearer access-4" {
				t.Fatal(got)
			}
			// Undefined credentials must fail before any token request.
			_, err := h.Send(context.Background(), Call{Request: req})
			if err == nil || grants.Load() != 4 {
				t.Fatal("undefined secret reached the endpoint")
			}
		})
	}
}

func TestOAuthAuthorizationCodePKCEAndRotatingRefreshAcrossRestarts(t *testing.T) {
	var grants, browsers atomic.Int32
	authorization := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/resource" {
			io.WriteString(w, r.Header.Get("Authorization"))
			return
		}
		r.ParseForm()
		n := grants.Add(1)
		switch n {
		case 1:
			a := <-authorization
			hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if a.Get("code_challenge_method") != "S256" || a.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(hash[:]) || len(r.Form.Get("code_verifier")) < 43 {
				t.Error("missing or incorrect S256 proof")
			}
			if r.Form.Get("code") != "one-use-code" || r.Form.Get("redirect_uri") != a.Get("redirect_uri") || r.Form.Get("grant_type") != "authorization_code" {
				t.Error("code exchange did not match authorization")
			}
		case 2, 3:
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != fmt.Sprintf("refresh-%d", n-1) {
				t.Errorf("refresh %d did not use the latest token", n)
			}
		default:
			t.Errorf("unexpected token acquisition %d", n)
		}
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			fmt.Fprintf(w, `{"access_token":"access-%d","refresh_token":"refresh-%d","expires_in":1,"token_type":"Bearer"}`, n, n)
		} else {
			io.WriteString(w, `{"access_token":"access-3","expires_in":3600,"token_type":"Bearer"}`)
		}
	}))
	defer server.Close()
	open := func(raw string) error {
		browsers.Add(1)
		u, _ := url.Parse(raw)
		a := u.Query()
		authorization <- a
		callback, _ := url.Parse(a.Get("redirect_uri"))
		q := url.Values{"state": {"wrong-state"}, "code": {"one-use-code"}}
		callback.RawQuery = q.Encode()
		bad, err := http.Get(callback.String())
		if err != nil {
			return err
		}
		bad.Body.Close()
		if bad.StatusCode != 400 {
			t.Error("callback accepted wrong state")
		}
		q.Set("state", a.Get("state"))
		callback.RawQuery = q.Encode()
		good, err := http.Get(callback.String())
		if err == nil {
			good.Body.Close()
		}
		return err
	}
	store := &memoryTokens{}
	req := oauthRequest(server.URL, "authorization_code")
	req.Auth.OAuth2.ClientSecret = "" // A public PKCE client.
	before, _ := collection.MarshalRequest(req)
	for n := 1; n <= 3; n++ {
		h := oauthHTTP(store, open)
		resp := send(t, h, req, nil)
		if got, want := string(resp.Body), fmt.Sprintf("Bearer access-%d", n); got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
	if browsers.Load() != 1 {
		t.Fatalf("browser opened %d times", browsers.Load())
	}
	for _, value := range store.values {
		var token struct {
			RefreshToken string `json:"refresh_token"`
		}
		json.Unmarshal([]byte(value), &token)
		if token.RefreshToken != "refresh-2" {
			t.Error("refresh response lost retained refresh token")
		}
	}
	after, _ := collection.MarshalRequest(req)
	if string(before) != string(after) || strings.Contains(string(after), "access-1") {
		t.Fatal("OAuth mutated the saved configuration")
	}
}

func TestOAuthVaultFailureAndSafeErrors(t *testing.T) {
	var grants atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/resource" {
			io.WriteString(w, "ok")
			return
		}
		grants.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			io.WriteString(w, `{"access_token":"sensitive-access","expires_in":3600}`)
		case "/bad":
			w.WriteHeader(400)
			io.WriteString(w, `{"error":"invalid_client","error_description":"sensitive-access"}`)
		case "/malformed":
			io.WriteString(w, `{"access_token":"sensitive-access","expires_in":"sensitive-access"}`)
		case "/redirect":
			w.Header().Set("Location", "/token")
			w.WriteHeader(307)
		}
	}))
	defer server.Close()
	store := &memoryTokens{err: errors.New("no vault")}
	h := oauthHTTP(store, nil)
	req := oauthRequest(server.URL, "client_credentials")
	req.Auth.OAuth2.ClientSecret = "secret"
	var status string
	for range 2 {
		_, err := h.Send(context.Background(), Call{Request: req, OnAuth: func(s string) { status = s }})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(status, "session only") {
			t.Fatalf("missing storage warning: %s", status)
		}
	}
	if grants.Load() != 1 || len(store.values) != 0 {
		t.Fatal("memory-only token was not reused")
	}
	for _, path := range []string{"/bad", "/malformed", "/redirect"} {
		req.Auth.OAuth2.TokenURL = server.URL + path
		before := grants.Load()
		_, err := h.Send(context.Background(), Call{Request: req})
		if err == nil || strings.Contains(err.Error(), "sensitive-access") {
			t.Fatalf("unsafe error: %v", err)
		}
		if grants.Load() != before+1 {
			t.Fatal("retried a grant or followed a token redirect")
		}
	}
	for _, raw := range []string{"http://identity.example.com/token", "https://user:secret@example.com/token"} {
		req.Auth.OAuth2.TokenURL = raw
		_, err := h.Send(context.Background(), Call{Request: req})
		if err == nil || strings.Contains(err.Error(), "secret@") {
			t.Fatalf("unsafe endpoint accepted: %v", err)
		}
	}
}

func TestOAuthBrowserCancellationAndDenial(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(fmt.Sprint(deny), func(t *testing.T) {
			opened := make(chan string, 1)
			h := oauthHTTP(&memoryTokens{}, func(raw string) error { opened <- raw; return nil })
			req := oauthRequest("https://example.com", "authorization_code")
			req.Auth.OAuth2.ClientSecret = ""
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := h.Send(ctx, Call{Request: req}); done <- err }()
			var raw string
			select {
			case raw = <-opened:
			case <-ctx.Done():
				t.Fatal("browser did not open")
			}
			u, _ := url.Parse(raw)
			callback := u.Query().Get("redirect_uri")
			if deny {
				resp, err := http.Get(callback + "?" + url.Values{"state": {u.Query().Get("state")}, "error": {"access_denied"}}.Encode())
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil || !strings.Contains(string(body), "Return to Posting") {
					t.Fatal("callback closed before confirming receipt in the browser")
				}
			} else {
				cancel()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled/denied authorization succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("callback listener did not stop")
			}
			resp, err := http.Get(callback)
			if err == nil {
				resp.Body.Close()
				t.Fatal("callback listener left open")
			}
		})
	}
}

func TestOAuthResourceRedirectsAndNoReplay(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("OAuth token leaked to a different origin")
		}
		io.WriteString(w, "other origin")
	}))
	defer other.Close()
	var rejected atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"origin-token","expires_in":3600}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer origin-token" {
			t.Error("original origin did not receive OAuth authorization")
		}
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/resource", http.StatusTemporaryRedirect)
		case "/other":
			http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
		case "/reject":
			rejected.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
		default:
			io.WriteString(w, "original origin")
		}
	}))
	defer server.Close()
	h := oauthHTTP(&memoryTokens{}, nil)
	req := oauthRequest(server.URL, "client_credentials")
	req.Auth.OAuth2.ClientSecret = "secret"
	req.Method = model.MethodPost
	req.Headers = []model.KeyValue{{Name: "Authorization", Value: "Bearer old", Enabled: true}}
	for _, tc := range []struct{ path, want string }{{"/same", "original origin"}, {"/other", "other origin"}} {
		req.URL = server.URL + tc.path
		if got := string(send(t, h, req, nil).Body); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.path, got, tc.want)
		}
	}
	req.URL = server.URL + "/reject"
	if resp := send(t, h, req, nil); resp.StatusCode != 401 || rejected.Load() != 1 {
		t.Fatal("an unauthorized POST was replayed or its response was lost")
	}
}

func TestOAuthGRPCSendAndNoninteractiveReflection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"grpc-token","expires_in":3600}`)
	}))
	defer server.Close()
	addr := startLibrary(t, libraryOptions{tls: true})
	req := grpcRequest("grpcs://"+addr, "library.v1.Library/GetBook", `{"isbn":"1"}`)
	req.Options.VerifySSL = false // The fixture uses a self-signed certificate.
	req.Auth = oauthRequest(server.URL, "client_credentials").Auth
	req.Auth.OAuth2.ClientSecret = "secret"
	g := NewGRPC("test", TLSSettings{}, "")
	g.OAuth = oauthHTTP(&memoryTokens{}, func(string) error { t.Error("reflection opened a browser"); return nil }).OAuth
	resp, err := g.Send(context.Background(), Call{Request: req})
	if err != nil {
		t.Fatal(err)
	}
	if got := headerValues(resp.Headers, "echo-authorization"); len(got) != 1 || got[0] != "Bearer grpc-token" {
		t.Fatalf("gRPC authorization = %v", got)
	}
	if schema, err := g.Describe(context.Background(), Call{Request: req}); err != nil || len(schema.Methods) == 0 {
		t.Fatalf("OAuth reflection: %+v, %v", schema, err)
	}
	req.Auth.OAuth2.GrantType = "authorization_code"
	if _, err := g.Describe(context.Background(), Call{Request: req}); err == nil || !strings.Contains(err.Error(), "send the request") {
		t.Fatalf("reflection without browser consent: %v", err)
	}
	req.URL = "grpc://" + addr
	if _, err := g.Send(context.Background(), Call{Request: req}); err == nil || !strings.Contains(err.Error(), "require TLS") {
		t.Fatalf("OAuth accepted plaintext gRPC: %v", err)
	}
}
