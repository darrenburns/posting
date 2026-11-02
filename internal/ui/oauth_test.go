package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/zalando/go-keyring"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/darrenburns/posting/v3/internal/remote"
)

func TestOAuthEditorSavesBothGrantsAndScrolls(tt *testing.T) {
	app := testApp()
	s := app.current()
	s.requestTab.Set("auth")
	p := t.NewPilot(tt, app, snapW, snapH)
	focus := func(id string) {
		t.RequestFocus(id)
		p.Advance(0)
		if p.FocusedID() != id {
			tt.Fatalf("could not focus %s", id)
		}
	}
	focus("req-auth-type")
	p.Press("left") // None wraps to OAuth 2.0.
	focus("req-oauth-authorization-url")
	p.Type("https://identity.example.com/authorize")
	focus("req-oauth-token-url")
	p.Type("https://identity.example.com/token")
	focus("req-oauth-client-id")
	p.Type("${CLIENT_ID}")
	focus("req-oauth-client-secret")
	p.Type("${CLIENT_SECRET}")
	focus("req-oauth-scope")
	p.Type("openid profile offline_access")
	focus("req-oauth-client-auth")
	p.Press("right")

	for _, grant := range []string{"authorization_code", "client_credentials"} {
		if grant == "client_credentials" {
			focus("req-oauth-grant")
			p.Press("right")
		}
		data, err := collection.MarshalRequest(s.Snapshot())
		if err != nil {
			tt.Fatal(err)
		}
		req, err := collection.ParseRequest(data, "")
		if err != nil {
			tt.Fatal(err)
		}
		o := req.Auth.OAuth2
		if req.Auth.Type != model.AuthOAuth2 || o.GrantType != grant || o.TokenURL != "https://identity.example.com/token" || o.AuthorizationURL != "https://identity.example.com/authorize" || o.ClientID != "${CLIENT_ID}" || o.ClientSecret != "${CLIENT_SECRET}" || o.Scope != "openid profile offline_access" || o.ClientAuth != "client_secret_basic" {
			tt.Fatalf("editor lost configuration: %+v", req.Auth)
		}
		p.Resize(90, 20)
		focus("req-oauth-client-auth")
		bounds, ok := p.Bounds("req-oauth-client-auth")
		if !ok || bounds.Y < 0 || bounds.Y >= 19 {
			tt.Fatalf("last OAuth control did not scroll into view: %+v, visible=%v\n%s", bounds, ok, p.ScreenText())
		}
		p.Resize(snapW, snapH)
	}
}

func TestRemoteOAuthKeepsTokensOutOfSavedRequestsAndHistory(tt *testing.T) {
	keyring.MockInit()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			r.ParseForm()
			if r.Form.Get("client_secret") != "demo-secret" {
				tt.Error("environment secret wasn't resolved")
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"private-access-token","refresh_token":"private-refresh-token","expires_in":3600}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer private-access-token" {
			tt.Error("resource request missing OAuth token")
		}
		io.WriteString(w, "authenticated")
	}))
	defer server.Close()
	h := newRemoteHarness(tt)
	h.app.sessionVars.Set(map[string]string{"CLIENT_SECRET": "demo-secret"})
	sender := client.NewHTTP("test", client.TLSSettings{})
	sender.OAuth = client.NewOAuth(tt.TempDir(), client.TLSSettings{}, nil)
	h.app.sender = sender
	h.app.oauth = sender.OAuth
	yaml := fmt.Sprintf(`url: %s/resource
auth:
  type: oauth2
  oauth2:
    grant_type: client_credentials
    token_url: %s/token
    client_id: demo-client
    client_secret: ${CLIENT_SECRET}
`, server.URL, server.URL)
	result := h.send(remote.Request{YAML: yaml})
	if result.Outcome != remote.OutcomeDone || result.Response == nil || result.Response.Body != "authenticated" {
		tt.Fatalf("OAuth remote result: %+v", result)
	}
	request, err := collection.MarshalRequest(h.app.current().Snapshot())
	if err != nil {
		tt.Fatal(err)
	}
	history, _ := json.Marshal(h.app.history.Peek())
	for _, secret := range []string{"private-access-token", "private-refresh-token", "demo-secret"} {
		if strings.Contains(string(request), secret) || strings.Contains(string(history), secret) {
			tt.Fatal("runtime secret leaked into request or history")
		}
	}
	if !strings.Contains(string(request), "${CLIENT_SECRET}") {
		tt.Fatal("saved request lost secret reference")
	}
	h.app.openExport()
	if h.app.curlDialog.text.GetText() != "" || !strings.Contains(h.app.curlDialog.err.Peek(), "OAuth tokens aren't exported") {
		tt.Fatal("OAuth curl export was not clearly refused")
	}
}
