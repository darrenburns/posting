package ui

import (
	"context"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
)

// oauthForm edits configuration only. Tokens stay in client.OAuth.
type oauthForm struct {
	grant, clientAuth                                                      t.Signal[string]
	authorizationURL, tokenURL, clientID, clientSecret, scope, redirectURI *t.TextInputState
	completions                                                            map[string]*completion
	status                                                                 t.Signal[string]
	shimmer                                                                *t.ShimmerState
}

func newOAuthForm() *oauthForm {
	return &oauthForm{
		grant: t.NewSignal("authorization_code"), clientAuth: t.NewSignal("client_secret_post"),
		authorizationURL: t.NewTextInputState(""), tokenURL: t.NewTextInputState(""),
		clientID: t.NewTextInputState(""), clientSecret: t.NewTextInputState(""),
		scope: t.NewTextInputState(""), redirectURI: t.NewTextInputState(""),
		completions: map[string]*completion{}, status: t.NewSignal(""),
		shimmer: t.NewShimmerState(0),
	}
}

func (f *oauthForm) load(o model.OAuth2) {
	o = o.WithDefaults()
	f.grant.Set(o.GrantType)
	f.clientAuth.Set(o.ClientAuth)
	f.authorizationURL.SetText(o.AuthorizationURL)
	f.tokenURL.SetText(o.TokenURL)
	f.clientID.SetText(o.ClientID)
	f.clientSecret.SetText(o.ClientSecret)
	f.scope.SetText(o.Scope)
	f.redirectURI.SetText(o.RedirectURI)
	f.status.Set("")
}

func (f *oauthForm) snapshot() model.OAuth2 {
	return model.OAuth2{GrantType: f.grant.Peek(), ClientAuth: f.clientAuth.Peek(),
		AuthorizationURL: f.authorizationURL.GetText(), TokenURL: f.tokenURL.GetText(),
		ClientID: f.clientID.GetText(), ClientSecret: f.clientSecret.GetText(),
		Scope: f.scope.GetText(), RedirectURI: f.redirectURI.GetText()}
}

func (e authEditor) oauthRows(ctx t.BuildContext) []formField {
	s, theme := e.session, ctx.Theme()
	f := s.oauth
	touch := func(string) { f.status.Set(""); s.touch() }
	rows := []formField{field(t.Column{Children: []t.Widget{
		t.Text{Content: "Grant", Style: t.Style{ForegroundColor: theme.TextMuted, Padding: inset}},
		tabStrip{ID: "req-oauth-grant", Tabs: []tabItem{{Key: "authorization_code", Label: "Code + PKCE"}, {Key: "client_credentials", Label: "Client credentials"}}, Active: f.grant.Get(),
			OnChange: func(v string) { f.grant.Set(v); touch(v) }},
	}}, "req-oauth-grant")}
	text := func(id, label, placeholder string, state *t.TextInputState) {
		id = "req-oauth-" + id
		if f.completions[id] == nil {
			f.completions[id] = newCompletion()
		}
		rows = append(rows, field(formRow(ctx, label, "", input{ID: id, State: state, Placeholder: placeholder,
			Highlighter: e.variables, OnChange: touch, Completion: f.completions[id], Choices: e.choices}), id))
	}
	if f.grant.Get() == "authorization_code" {
		text("authorization-url", "Auth URL", "https://identity.example.com/authorize", f.authorizationURL)
	}
	text("token-url", "Token URL", "https://identity.example.com/token", f.tokenURL)
	text("client-id", "Client ID", "${CLIENT_ID}", f.clientID)
	text("client-secret", "Client secret", "${CLIENT_SECRET} — omit for public clients", f.clientSecret)
	text("scope", "Scopes", "Space-separated scopes, e.g. openid profile offline_access", f.scope)
	if f.grant.Get() == "authorization_code" {
		text("redirect", "Redirect URI", "http://127.0.0.1:8765/oauth/callback", f.redirectURI)
	}
	rows = append(rows,
		field(formRow(ctx, "Client auth", "", tabStrip{ID: "req-oauth-client-auth", Tabs: []tabItem{{Key: "client_secret_post", Label: "Body"}, {Key: "client_secret_basic", Label: "Basic header"}}, Active: f.clientAuth.Get(),
			OnChange: func(v string) { f.clientAuth.Set(v); touch(v) }}), "req-oauth-client-auth"),
		field(t.Text{Content: "Send to fetch or refresh a token. Code grants open your browser.\nUse variables for secrets; literal credentials are saved in request YAML.", Style: t.Style{ForegroundColor: theme.TextMuted, Padding: inset}}),
		field(t.Text{Content: f.status.Get(), Style: t.Style{ForegroundColor: theme.WarningText, Padding: inset}}),
	)
	return rows
}

func (a *App) forgetOAuth() {
	s := a.current()
	if s == nil || s.authType.Peek() != model.AuthOAuth2 || a.oauth == nil {
		return
	}
	if s.phase.Peek() == exchangeSending {
		a.notify("Cancel the request before forgetting its OAuth token", toastWarning)
		return
	}
	req := s.Snapshot()
	variables, err := a.requestVariableValues(req)
	if err != nil {
		a.notify(err.Error(), toastWarning)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.oauth.Forget(ctx, client.Call{Request: req, Variables: variables})
		a.dispatch(func() {
			if err != nil {
				s.oauth.status.Set(err.Error())
				a.notify(err.Error(), toastWarning)
				return
			}
			s.oauth.status.Set("Token forgotten. Send to authorize again.")
			a.notify("OAuth token forgotten", toastSuccess)
		})
	}()
}
