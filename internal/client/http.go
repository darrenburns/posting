package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/darrenburns/posting/internal/model"
)

// maxBodyBytes caps how much of a response body is read into memory.
const maxBodyBytes = 64 << 20

// TLSSettings configure certificates for every request.
type TLSSettings struct {
	// CABundle is a PEM file of extra certificate authorities to trust.
	CABundle string
	// CertFile and KeyFile are a client certificate and its private key, in
	// PEM. KeyFile may be empty when the certificate file also holds the key.
	CertFile string
	KeyFile  string
}

// HTTP sends requests over the network with net/http.
//
// Cookies set by responses are kept for the life of the client and attached
// to later requests that have the Attach cookies option on.
type HTTP struct {
	// UserAgent is sent when the request doesn't set one.
	UserAgent string
	TLS       TLSSettings

	once   sync.Once
	jar    http.CookieJar
	tlsErr error
	roots  *x509.CertPool
	certs  []tls.Certificate
}

// NewHTTP returns a client that identifies itself as userAgent.
func NewHTTP(userAgent string, settings TLSSettings) *HTTP {
	return &HTTP{UserAgent: userAgent, TLS: settings}
}

func (h *HTTP) init() {
	h.once.Do(func() {
		h.jar, _ = cookiejar.New(nil)
		if h.TLS.CABundle != "" {
			pem, err := os.ReadFile(h.TLS.CABundle)
			if err != nil {
				h.tlsErr = fmt.Errorf("reading CA bundle: %w", err)
				return
			}
			roots, err := x509.SystemCertPool()
			if err != nil || roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(pem) {
				h.tlsErr = fmt.Errorf("no certificates found in CA bundle %s", h.TLS.CABundle)
				return
			}
			h.roots = roots
		}
		if h.TLS.CertFile != "" {
			keyFile := h.TLS.KeyFile
			if keyFile == "" {
				keyFile = h.TLS.CertFile
			}
			cert, err := tls.LoadX509KeyPair(h.TLS.CertFile, keyFile)
			if err != nil {
				h.tlsErr = fmt.Errorf("loading client certificate: %w", err)
				return
			}
			h.certs = []tls.Certificate{cert}
		}
	})
}

// Send performs the request described by call.
func (h *HTTP) Send(ctx context.Context, call Call) (*model.Response, error) {
	h.init()
	if h.tlsErr != nil {
		return nil, h.tlsErr
	}
	req, err := model.Resolve(call.Request, call.Lookup)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", req.URL, err)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: no host", req.URL)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", target.Scheme)
	}

	timeout := req.Options.TimeoutSeconds
	if timeout <= 0 {
		timeout = model.DefaultOptions().TimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	defer cancel()

	transport, err := h.transport(req)
	if err != nil {
		return nil, err
	}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport}
	if req.Options.AttachCookies {
		httpClient.Jar = h.jar
	}
	if !req.Options.FollowRedirects {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	body, contentType := encodeBody(req.Body)
	tracer := newTracer(call.OnTrace, target.Scheme == "https")
	started := time.Now()
	build := func(authorization string) (*http.Request, error) {
		r, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, tracer.clientTrace()), string(req.Method), req.URL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if len(body) == 0 {
			r.Body, r.ContentLength = http.NoBody, 0
		}
		for _, hdr := range req.Headers {
			if hdr.Enabled && hdr.Name != "" {
				r.Header.Add(hdr.Name, hdr.Value)
			}
		}
		if contentType != "" && r.Header.Get("Content-Type") == "" {
			r.Header.Set("Content-Type", contentType)
		}
		if r.Header.Get("User-Agent") == "" && h.UserAgent != "" {
			r.Header.Set("User-Agent", h.UserAgent)
		}
		switch req.Auth.Type {
		case model.AuthBasic:
			r.SetBasicAuth(req.Auth.Username, req.Auth.Password)
		case model.AuthBearer:
			r.Header.Set("Authorization", "Bearer "+req.Auth.Token)
		}
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		return r, nil
	}

	httpReq, err := build("")
	if err != nil {
		return nil, err
	}
	tracer.start(model.TraceConnect)
	resp, err := httpClient.Do(httpReq)
	if err == nil && req.Auth.Type == model.AuthDigest && resp.StatusCode == http.StatusUnauthorized {
		if challenge, ok := parseDigestChallenge(resp.Header.Values("WWW-Authenticate")); ok {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
			resp.Body.Close()
			authorization, authErr := challenge.authorize(string(req.Method), resp.Request.URL, req.Auth.Username, req.Auth.Password, body)
			if authErr != nil {
				return nil, authErr
			}
			httpReq, err = build(authorization)
			if err == nil {
				resp, err = httpClient.Do(httpReq)
			}
		}
	}
	if err != nil {
		tracer.fail()
		return nil, describeError(ctx, err, timeout)
	}
	defer resp.Body.Close()
	tracer.complete(model.TraceReceiveHeaders)

	tracer.start(model.TraceReceiveBody)
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		tracer.fail()
		return nil, describeError(ctx, err, timeout)
	}
	raw = decompress(raw, resp.Header.Get("Content-Encoding"))
	tracer.complete(model.TraceReceiveBody)
	tracer.start(model.TraceClosed)
	resp.Body.Close()
	tracer.complete(model.TraceClosed)

	return &model.Response{
		StatusCode: resp.StatusCode,
		Reason:     reason(resp),
		Proto:      resp.Proto,
		Headers:    headers(resp.Header),
		Cookies:    cookies(resp.Cookies()),
		Body:       raw,
		Elapsed:    time.Since(started),
		ReceivedAt: time.Now(),
		Trace:      tracer.events(),
		URL:        resp.Request.URL.String(),
		Method:     req.Method,
	}, nil
}

func (h *HTTP) transport(req model.Request) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: !req.Options.VerifySSL, //nolint:gosec // The user's choice, per request.
		RootCAs:            h.roots,
		Certificates:       h.certs,
	}
	// Bodies are decompressed below, whatever Accept-Encoding the user
	// sent, so the transport's own gzip handling isn't needed.
	transport.DisableCompression = true
	if proxy := strings.TrimSpace(req.Options.ProxyURL); proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil || proxyURL.Host == "" {
			return nil, fmt.Errorf("invalid proxy URL %q", proxy)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return transport, nil
}

// encodeBody returns the bytes to send and the Content-Type they imply.
func encodeBody(body model.Body) ([]byte, string) {
	switch body.Type {
	case model.BodyRaw:
		return []byte(body.Raw), body.ContentType
	case model.BodyForm:
		values := url.Values{}
		var order []string
		for _, f := range body.Form {
			if f.Enabled && f.Name != "" {
				if _, seen := values[f.Name]; !seen {
					order = append(order, f.Name)
				}
				values.Add(f.Name, f.Value)
			}
		}
		// url.Values.Encode sorts by key; keep the order the user wrote.
		var parts []string
		for _, name := range order {
			for _, value := range values[name] {
				parts = append(parts, url.QueryEscape(name)+"="+url.QueryEscape(value))
			}
		}
		return []byte(strings.Join(parts, "&")), "application/x-www-form-urlencoded"
	}
	return nil, ""
}

// decompress undoes gzip or deflate encoding. Other encodings, and bodies
// that fail to decode, are returned as they arrived.
func decompress(body []byte, encoding string) []byte {
	var reader io.ReadCloser
	var err error
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip", "x-gzip":
		reader, err = gzip.NewReader(bytes.NewReader(body))
	case "deflate":
		reader = flate.NewReader(bytes.NewReader(body))
	default:
		return body
	}
	if err != nil {
		return body
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes))
	if err != nil {
		return body
	}
	return decoded
}

func reason(resp *http.Response) string {
	if _, text, ok := strings.Cut(resp.Status, " "); ok && text != "" {
		return text
	}
	return http.StatusText(resp.StatusCode)
}

// headers flattens the response headers, sorted by name since net/http
// doesn't keep the order they arrived in.
func headers(h http.Header) []model.Header {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []model.Header
	for _, name := range names {
		for _, value := range h[name] {
			out = append(out, model.Header{Name: name, Value: value})
		}
	}
	return out
}

func cookies(in []*http.Cookie) []model.Cookie {
	out := make([]model.Cookie, 0, len(in))
	for _, c := range in {
		cookie := model.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, HTTPOnly: c.HttpOnly, Secure: c.Secure}
		if !c.Expires.IsZero() {
			cookie.Expires = c.Expires.UTC().Format(time.RFC1123)
		}
		out = append(out, cookie)
	}
	return out
}

// describeError turns transport errors into messages worth showing.
func describeError(ctx context.Context, err error, timeout float64) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("request timed out after %gs", timeout)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Errorf("couldn't resolve host %q", dnsErr.Name)
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return fmt.Errorf("couldn't connect to %s: %v", opErr.Addr, opErr.Err)
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return fmt.Errorf("SSL certificate verification failed: %v (turn off Verify SSL in Options to skip it)", certErr.Err)
	}
	return err
}

// tracer turns httptrace callbacks into Posting's trace stages. Callbacks
// come from the transport's goroutines, so it locks.
type tracer struct {
	mu       sync.Mutex
	report   func(model.TraceEvent)
	https    bool
	started  map[model.TraceStage]time.Time
	finished map[model.TraceStage]model.TraceEvent
}

func newTracer(report func(model.TraceEvent), https bool) *tracer {
	return &tracer{
		report:   report,
		https:    https,
		started:  map[model.TraceStage]time.Time{},
		finished: map[model.TraceStage]model.TraceEvent{},
	}
}

func (t *tracer) emit(event model.TraceEvent) {
	if t.report != nil {
		t.report(event)
	}
}

func (t *tracer) start(stage model.TraceStage) {
	t.mu.Lock()
	t.started[stage] = time.Now()
	t.mu.Unlock()
	t.emit(model.TraceEvent{Stage: stage, State: model.TraceStarted})
}

func (t *tracer) complete(stage model.TraceStage) {
	t.mu.Lock()
	began, ok := t.started[stage]
	if !ok {
		began = time.Now()
	}
	event := model.TraceEvent{Stage: stage, State: model.TraceComplete, Duration: time.Since(began)}
	t.finished[stage] = event
	t.mu.Unlock()
	t.emit(event)
}

func (t *tracer) skip(stage model.TraceStage) {
	event := model.TraceEvent{Stage: stage, State: model.TraceSkipped}
	t.mu.Lock()
	t.finished[stage] = event
	t.mu.Unlock()
	t.emit(event)
}

// fail marks the stage in progress as failed.
func (t *tracer) fail() {
	t.mu.Lock()
	var failed []model.TraceStage
	for _, stage := range model.TraceStages {
		if _, began := t.started[stage]; began {
			if _, done := t.finished[stage]; !done {
				failed = append(failed, stage)
			}
		}
	}
	t.mu.Unlock()
	for _, stage := range failed {
		t.emit(model.TraceEvent{Stage: stage, State: model.TraceFailed})
	}
}

func (t *tracer) events() []model.TraceEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []model.TraceEvent
	for _, stage := range model.TraceStages {
		if event, ok := t.finished[stage]; ok {
			out = append(out, event)
		}
	}
	return out
}

func (t *tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				// No new connection: nothing to connect or handshake.
				t.complete(model.TraceConnect)
				if t.https {
					t.skip(model.TraceTLS)
				}
			}
			if !t.https {
				t.skip(model.TraceTLS)
			}
			t.start(model.TraceSendHeaders)
		},
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				t.complete(model.TraceConnect)
			}
		},
		TLSHandshakeStart: func() { t.start(model.TraceTLS) },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil {
				t.complete(model.TraceTLS)
			}
		},
		WroteHeaders: func() {
			t.complete(model.TraceSendHeaders)
			t.start(model.TraceSendBody)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				t.complete(model.TraceSendBody)
				t.start(model.TraceReceiveHeaders)
			}
		},
	}
}
