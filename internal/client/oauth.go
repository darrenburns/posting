package client

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/darrenburns/posting/v3/internal/model"
)

// OAuth owns runtime tokens separately from requests, exports and history.
// One instance is shared by the collection's HTTP and gRPC clients.
type OAuth struct {
	root    string
	http    *HTTP
	openURL func(string) error
	store   tokenStore
	mu      sync.Mutex
	entries map[string]*oauthEntry
}

type tokenStore interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}

type keyringTokens struct{}

func (keyringTokens) Get(key string) (string, error) { return keyring.Get("posting.oauth2", key) }
func (keyringTokens) Set(key, value string) error    { return keyring.Set("posting.oauth2", key, value) }
func (keyringTokens) Delete(key string) error        { return keyring.Delete("posting.oauth2", key) }

type oauthEntry struct {
	// A cancellable lock prevents concurrent sends from spending the same
	// rotating refresh token twice, without blocking unrelated logins.
	lock    chan struct{}
	loaded  bool
	token   *oauth2.Token
	warning string
}

func NewOAuth(root string, tls TLSSettings, openURL func(string) error) *OAuth {
	return &OAuth{root: root, http: NewHTTP("posting", tls), openURL: openURL, store: keyringTokens{}, entries: map[string]*oauthEntry{}}
}

func (o *OAuth) entry(ctx context.Context, req model.Request) (string, *oauthEntry, error) {
	// Share tokens only within a collection, resource origin and resolved
	// configuration. Switching credentials, endpoints or scope cannot reuse
	// another environment's token. Neither keys nor values contain a secret
	// outside the credential store.
	origin := req.URL
	if req.Kind().OverHTTP() || req.Kind() == model.WebSocketKind {
		u, err := url.Parse(req.URL)
		if err != nil {
			return "", nil, errors.New("invalid OAuth resource URL")
		}
		origin = u.Scheme + "://" + u.Host
	} else if g, ok := req.Payload.(model.GRPC); ok {
		origin += "|" + g.Authority
	}
	data, _ := json.Marshal([]any{o.root, origin, req.Auth.OAuth2.WithDefaults(), req.Options.VerifySSL, req.Options.ProxyURL})
	key := fmt.Sprintf("%x", sha256.Sum256(data))
	o.mu.Lock()
	e := o.entries[key]
	if e == nil {
		e = &oauthEntry{lock: make(chan struct{}, 1)}
		o.entries[key] = e
	}
	o.mu.Unlock()
	select {
	case e.lock <- struct{}{}:
		return key, e, nil
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

const vaultWarning = "OS credential store unavailable: OAuth token is kept for this session only."

// authorize accepts an already resolved request and returns wire-only auth.
// interactive is false during gRPC reflection: focusing a field must never
// open a browser. Every acquisition uses this call's context, not a cancelled
// context retained from an earlier send.
func (o *OAuth) authorize(ctx context.Context, req model.Request, interactive bool, report func(string)) (model.Auth, error) {
	if req.Auth.Type != model.AuthOAuth2 {
		return req.Auth, nil
	}
	if o == nil {
		return model.Auth{}, errors.New("OAuth 2.0 is not configured")
	}
	note := func(s string) {
		if report != nil {
			report(s)
		}
	}
	cfg, err := oauthConfig(req.Auth.OAuth2)
	if err != nil {
		return model.Auth{}, err
	}
	if req.Kind().OverHTTP() || req.Kind() == model.WebSocketKind {
		resource := req.URL
		if req.Kind() == model.WebSocketKind {
			resource = "http" + strings.TrimPrefix(resource, "ws")
		}
		if err := secureOAuthURL(resource); err != nil {
			return model.Auth{}, errors.New("OAuth requests require HTTPS (HTTP is allowed only on loopback)")
		}
	} else {
		target, err := model.ParseGRPCTarget(req.URL)
		if err != nil {
			return model.Auth{}, err
		}
		if !target.TLS {
			return model.Auth{}, errors.New("OAuth gRPC requests require TLS")
		}
	}
	key, e, err := o.entry(ctx, req)
	if err != nil {
		return model.Auth{}, err
	}
	defer func() { <-e.lock }()
	if !e.loaded {
		value, err := o.store.Get(key)
		switch {
		case err == nil:
			if err := json.Unmarshal([]byte(value), &e.token); err != nil {
				return model.Auth{}, errors.New("stored OAuth token is unreadable; use Forget OAuth token to reset it")
			}
		case errors.Is(err, keyring.ErrNotFound):
		default:
			e.warning = vaultWarning
		}
		e.loaded = true
	}
	if !e.token.Valid() {
		o.http.init()
		if o.http.tlsErr != nil {
			return model.Auth{}, o.http.tlsErr
		}
		transport, err := o.http.transport(req)
		if err != nil {
			return model.Auth{}, err
		}
		defer transport.CloseIdleConnections()
		// Never follow token endpoint redirects: a 307 could forward the
		// client secret, authorization code or refresh token to another host.
		hc := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
		var token *oauth2.Token
		grant := req.Auth.OAuth2.WithDefaults().GrantType
		switch {
		case grant == "client_credentials":
			note("Fetching OAuth token…")
			cc := clientcredentials.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, TokenURL: cfg.Endpoint.TokenURL, Scopes: cfg.Scopes, AuthStyle: cfg.Endpoint.AuthStyle}
			token, err = cc.Token(ctx)
			err = oauthError(err)
		case e.token != nil && e.token.RefreshToken != "":
			note("Refreshing OAuth token…")
			token, err = cfg.TokenSource(ctx, e.token).Token()
			err = oauthError(err)
		case !interactive:
			return model.Auth{}, errors.New("send the request to authorize OAuth in your browser first")
		default:
			token, err = o.browserToken(ctx, cfg, note)
		}
		if err != nil {
			return model.Auth{}, err
		}
		if token.AccessToken == "" || (token.TokenType != "" && !strings.EqualFold(token.TokenType, "Bearer")) || strings.ContainsAny(token.AccessToken, "\r\n") {
			return model.Auth{}, errors.New("OAuth endpoint did not return a usable Bearer token")
		}
		e.token = token
		data, err := json.Marshal(token)
		if err == nil {
			err = o.store.Set(key, string(data))
		}
		e.warning = ""
		if err != nil {
			e.warning = vaultWarning
		}
	}
	if e.warning != "" {
		note(e.warning)
	} else {
		note("OAuth token stored in the OS credential store.")
	}
	return model.Auth{Type: model.AuthBearer, Token: e.token.AccessToken}, nil
}

// Forget clears this configuration's cached credentials, not the provider's
// consent. Failed vault deletion is reported rather than claiming logout.
func (o *OAuth) Forget(ctx context.Context, call Call) error {
	req, err := model.Resolve(call.Request, call.Lookup)
	if err != nil {
		return err
	}
	key, e, err := o.entry(ctx, req)
	if err != nil {
		return err
	}
	defer func() { <-e.lock }()
	e.token, e.loaded = nil, true
	if err := o.store.Delete(key); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return errors.New("session token cleared, but the OS credential store could not delete the saved token")
	}
	e.warning = ""
	return nil
}

func oauthConfig(o model.OAuth2) (*oauth2.Config, error) {
	o = o.WithDefaults()
	if o.GrantType != "authorization_code" && o.GrantType != "client_credentials" {
		return nil, errors.New("OAuth grant must be authorization_code or client_credentials")
	}
	if strings.TrimSpace(o.ClientID) == "" {
		return nil, errors.New("OAuth client ID is required")
	}
	if err := secureOAuthURL(o.TokenURL); err != nil {
		return nil, errors.New("OAuth token URL must use HTTPS (HTTP is allowed only on loopback)")
	}
	style := oauth2.AuthStyleInParams
	switch o.ClientAuth {
	case "client_secret_basic":
		style = oauth2.AuthStyleInHeader
	case "client_secret_post":
	default:
		return nil, errors.New("OAuth client authentication must be client_secret_basic or client_secret_post")
	}
	if o.GrantType == "client_credentials" && o.ClientSecret == "" {
		return nil, errors.New("OAuth client secret is required for client credentials")
	}
	if o.GrantType == "authorization_code" {
		if err := secureOAuthURL(o.AuthorizationURL); err != nil {
			return nil, errors.New("OAuth authorization URL must use HTTPS (HTTP is allowed only on loopback)")
		}
		if _, err := oauthRedirect(o.RedirectURI); err != nil {
			return nil, err
		}
	}
	return &oauth2.Config{ClientID: o.ClientID, ClientSecret: o.ClientSecret, Scopes: strings.Fields(o.Scope), RedirectURL: o.RedirectURI,
		Endpoint: oauth2.Endpoint{AuthURL: o.AuthorizationURL, TokenURL: o.TokenURL, AuthStyle: style}}, nil
}

func secureOAuthURL(raw string) error {
	u, err := url.Parse(raw)
	if err == nil && u.Hostname() != "" && u.User == nil && u.Fragment == "" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()) {
			return nil
		}
	}
	return errors.New("HTTPS required")
}

func oauthRedirect(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err == nil {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme == "http" && ip != nil && ip.IsLoopback() && u.Port() != "" && u.Path != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
			return u, nil
		}
	}
	return nil, errors.New("OAuth redirect URI must be an HTTP loopback IP with a port and path, e.g. http://127.0.0.1:8765/oauth/callback")
}

func (o *OAuth) browserToken(ctx context.Context, cfg *oauth2.Config, note func(string)) (*oauth2.Token, error) {
	if o.openURL == nil {
		return nil, errors.New("no browser launcher available for OAuth authorization")
	}
	redirect, err := oauthRedirect(cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return nil, errors.New("could not listen on the OAuth callback port; check the redirect URI or close the other login")
	}
	defer listener.Close()
	// Port 0 is useful for providers that allow an ephemeral loopback port.
	redirect.Host = listener.Addr().String()
	config := *cfg
	config.RedirectURL = redirect.String()
	state, verifier := oauth2.GenerateVerifier(), oauth2.GenerateVerifier()
	type result struct {
		code string
		err  error
	}
	callback := make(chan result, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		q := r.URL.Query()
		if r.Method != "GET" || r.Host != redirect.Host || r.URL.Path != redirect.Path {
			http.NotFound(w, r)
			return
		}
		if len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid OAuth state. Return to the original authorization page.", http.StatusBadRequest)
			return
		}
		out := result{code: q.Get("code")}
		if q.Get("error") != "" {
			out.err = errors.New("OAuth authorization was denied; send again to retry")
		} else if len(q["code"]) != 1 || out.code == "" {
			http.Error(w, "Missing authorization code.", http.StatusBadRequest)
			return
		}
		select {
		case callback <- out:
		default:
		}
		io.WriteString(w, "Authorization response received. Return to Posting.")
	})}
	go server.Serve(listener)
	defer func() {
		// Let the callback finish its browser response before closing the
		// listener, including when consent was denied without a token exchange.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
		server.Close()
	}()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	authURL := config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.AccessTypeOffline)
	note("Waiting for OAuth authorization in your browser… (Esc cancels)")
	opened := make(chan error, 1)
	go func() { opened <- o.openURL(authURL) }()
	for {
		select {
		case err := <-opened:
			if err != nil {
				return nil, errors.New("could not open the browser for OAuth; check your system browser configuration")
			}
		case out := <-callback:
			if out.err != nil {
				return nil, out.err
			}
			note("Exchanging OAuth authorization code…")
			token, err := config.Exchange(ctx, out.code, oauth2.VerifierOption(verifier))
			return token, oauthError(err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// RetrieveError.Error includes the token endpoint body, which may contain
// credentials. Neither it nor a URL-bearing transport error reaches the UI.
func oauthError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.New("OAuth authorization cancelled or timed out; send again to retry")
	}
	var response *oauth2.RetrieveError
	if errors.As(err, &response) {
		if response.ErrorCode == "invalid_grant" {
			return errors.New("OAuth grant expired or was revoked; use Forget OAuth token and sign in again")
		}
		return fmt.Errorf("OAuth token endpoint rejected the request (HTTP %d); check the client credentials, scope and client authentication method", response.Response.StatusCode)
	}
	var transport *url.Error
	if errors.As(err, &transport) {
		return errors.New("OAuth token endpoint could not be reached; check its URL, proxy and TLS settings")
	}
	return errors.New("OAuth token endpoint returned an invalid response")
}
