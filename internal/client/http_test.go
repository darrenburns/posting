package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/model"
)

// echo replies with what it received, as JSON.
func echo(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	user, pass, _ := r.BasicAuth()
	var cookies []string
	for _, c := range r.Cookies() {
		cookies = append(cookies, c.Name+"="+c.Value)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"method":  r.Method,
		"path":    r.URL.Path,
		"query":   r.URL.RawQuery,
		"body":    string(body),
		"ctype":   r.Header.Get("Content-Type"),
		"auth":    r.Header.Get("Authorization"),
		"user":    user,
		"pass":    pass,
		"ua":      r.Header.Get("User-Agent"),
		"x":       r.Header.Get("X-Test"),
		"cookies": strings.Join(cookies, ";"),
	})
}

func decode(t *testing.T, resp *model.Response) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("decoding %q: %v", resp.Body, err)
	}
	return out
}

func send(t *testing.T, client *HTTP, req model.Request, vars map[string]string) *model.Response {
	t.Helper()
	resp, err := client.Send(context.Background(), Call{Request: req, Variables: vars})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	return resp
}

func TestHTTPSendsTheResolvedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()
	client := NewHTTP("posting-test", TLSSettings{})

	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = "${BASE}/users/:id?q=${TERM}&plain=1"
	req.PathParams = []model.KeyValue{{Name: "id", Value: "${ID}", Enabled: true}}
	req.Headers = []model.KeyValue{
		{Name: "X-Test", Value: "hello ${NAME}", Enabled: true},
		{Name: "X-Off", Value: "no", Enabled: false},
	}
	req.Body = model.Body{Type: model.BodyRaw, Raw: `{"name": "${NAME}", "cost": "$$5"}`, ContentType: "application/json"}
	req.Auth = model.Auth{Type: model.AuthBearer, Token: "${TOKEN}"}
	vars := map[string]string{"BASE": server.URL, "ID": "42", "TERM": "a&b c", "NAME": "Ada", "TOKEN": "t0k"}

	resp := send(t, client, req, vars)
	got := decode(t, resp)
	want := map[string]any{
		"method": "POST", "path": "/users/42", "query": "q=a%26b+c&plain=1",
		"body": `{"name": "Ada", "cost": "$5"}`, "ctype": "application/json",
		"auth": "Bearer t0k", "x": "hello Ada", "ua": "posting-test",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}
	if resp.StatusCode != 200 || resp.Reason != "OK" || resp.Method != model.MethodPost {
		t.Errorf("status = %d %q", resp.StatusCode, resp.Reason)
	}
	if !strings.HasPrefix(resp.URL, server.URL+"/users/42?") {
		t.Errorf("URL = %q", resp.URL)
	}

	// With substitution off, the body goes out exactly as written.
	req.Options.SubstituteBodyVariables = false
	got = decode(t, send(t, client, req, vars))
	if got["body"] != `{"name": "${NAME}", "cost": "$$5"}` {
		t.Errorf("literal body = %q", got["body"])
	}
}

func TestHTTPFormBodyAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()
	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = server.URL + "/login"
	req.Body = model.Body{Type: model.BodyForm, Form: []model.KeyValue{
		{Name: "user", Value: "ada", Enabled: true},
		{Name: "skip", Value: "x", Enabled: false},
		{Name: "pw", Value: "p&ss", Enabled: true},
	}}
	req.Auth = model.Auth{Type: model.AuthBasic, Username: "ada", Password: "secret"}
	got := decode(t, send(t, NewHTTP("", TLSSettings{}), req, nil))
	if got["body"] != "user=ada&pw=p%26ss" || got["ctype"] != "application/x-www-form-urlencoded" {
		t.Errorf("form body = %q (%q)", got["body"], got["ctype"])
	}
	if got["user"] != "ada" || got["pass"] != "secret" {
		t.Errorf("basic auth = %q:%q", got["user"], got["pass"])
	}
}

func TestHTTPDigestAuth(t *testing.T) {
	const realm, nonce = "test", "abc123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := parseAuthParams(strings.TrimPrefix(r.Header.Get("Authorization"), "Digest "))
		md5hex := func(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }
		ha1 := md5hex("ada:" + realm + ":secret")
		ha2 := md5hex(r.Method + ":" + params["uri"])
		expected := md5hex(strings.Join([]string{ha1, nonce, params["nc"], params["cnonce"], "auth", ha2}, ":"))
		if params["response"] != expected {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth", algorithm=MD5`, realm, nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, "welcome")
	}))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL + "/private?x=1"
	req.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
	resp := send(t, NewHTTP("", TLSSettings{}), req, nil)
	if resp.StatusCode != 200 || string(resp.Body) != "welcome" {
		t.Fatalf("digest auth failed: %d %s", resp.StatusCode, resp.Body)
	}
	req.Auth.Password = "wrong"
	if resp := send(t, NewHTTP("", TLSSettings{}), req, nil); resp.StatusCode != 401 {
		t.Fatalf("wrong password got %d", resp.StatusCode)
	}
}

func TestHTTPRedirectsAndCookies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1", Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/end", http.StatusFound)
	})
	mux.HandleFunc("/end", echo)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewHTTP("", TLSSettings{})

	req := model.NewRequest()
	req.URL = server.URL + "/start"
	resp := send(t, client, req, nil)
	if got := decode(t, resp); got["path"] != "/end" || got["cookies"] != "session=s1" {
		t.Errorf("followed redirect = %v", got)
	}

	req.Options.FollowRedirects = false
	resp = send(t, client, req, nil)
	if resp.StatusCode != http.StatusFound || len(resp.Cookies) != 1 || !resp.Cookies[0].HTTPOnly {
		t.Errorf("unfollowed redirect: %d, cookies %+v", resp.StatusCode, resp.Cookies)
	}

	// Cookies from earlier responses are only attached when asked for.
	req.URL = server.URL + "/end"
	req.Options.AttachCookies = false
	if got := decode(t, send(t, client, req, nil)); got["cookies"] != "" {
		t.Errorf("cookies attached with the option off: %v", got["cookies"])
	}
}

func TestHTTPDecompressesGzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write([]byte(`{"ok": true}`))
		zw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	req.Headers = []model.KeyValue{{Name: "Accept-Encoding", Value: "gzip", Enabled: true}}
	if resp := send(t, NewHTTP("", TLSSettings{}), req, nil); string(resp.Body) != `{"ok": true}` {
		t.Fatalf("body = %q", resp.Body)
	}
}

func TestHTTPTimeoutAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	client := NewHTTP("", TLSSettings{})
	req := model.NewRequest()
	req.URL = server.URL
	req.Options.TimeoutSeconds = 0.1
	_, err := client.Send(context.Background(), Call{Request: req})
	if err == nil || !strings.Contains(err.Error(), "timed out after 0.1s") {
		t.Errorf("timeout error = %v", err)
	}

	req.URL = "${MISSING}/x"
	_, err = client.Send(context.Background(), Call{Request: req})
	if err == nil || err.Error() != "Variable not defined: $MISSING" {
		t.Errorf("undefined variable error = %v", err)
	}
}

func TestHTTPReportsTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()
	req := model.NewRequest()
	req.URL = server.URL
	var reported []model.TraceEvent
	done := make(chan struct{})
	var resp *model.Response
	go func() {
		defer close(done)
		resp, _ = NewHTTP("", TLSSettings{}).Send(context.Background(), Call{Request: req, OnTrace: func(e model.TraceEvent) { reported = append(reported, e) }})
	}()
	<-done
	if resp == nil {
		t.Fatal("no response")
	}
	states := map[model.TraceStage]model.TraceState{}
	for _, e := range resp.Trace {
		states[e.Stage] = e.State
	}
	for _, stage := range model.TraceStages {
		want := model.TraceComplete
		if stage == model.TraceTLS {
			want = model.TraceSkipped
		}
		if states[stage] != want {
			t.Errorf("%s = %v, want %v", stage, states[stage], want)
		}
	}
	if len(reported) == 0 {
		t.Error("no trace progress reported")
	}
}

func TestResolveAddsSchemeAndQueryParams(t *testing.T) {
	req := model.NewRequest()
	req.URL = "localhost:8080/items"
	req.Query = []model.KeyValue{{Name: "a", Value: "1", Enabled: true}, {Name: "b", Value: "2", Enabled: false}}
	out, err := model.Resolve(req, model.MapLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out.URL != "http://localhost:8080/items?a=1" {
		t.Fatalf("URL = %q", out.URL)
	}
}
