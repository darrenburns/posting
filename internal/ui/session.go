package ui

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

// exchangePhase is where a session's current send is up to.
type exchangePhase int

const (
	exchangeIdle exchangePhase = iota
	exchangeSending
	exchangeDone
	exchangeFailed
	exchangeCancelled
)

// Session is one open request tab: the editor state for the request plus the
// state of its most recent exchange. All mutation happens on the UI goroutine.
//
// Snapshot and Load are the only bridge between the editor widgets and
// model.Request, so the rest of the app (and any future HTTP client) only
// ever deals in plain model values.
type Session struct {
	id int

	// Request editing state.
	method      t.Signal[model.Method]
	url         *t.TextInputState
	headers     *kvEditor
	query       *kvEditor
	pathParams  *kvEditor
	form        *kvEditor
	bodyType    t.Signal[model.BodyType]
	contentType t.Signal[string]
	body        *t.TextAreaState
	bodyScroll  *t.ScrollState
	authType    t.Signal[model.AuthType]
	username    *t.TextInputState
	password    *t.TextInputState
	token       *t.TextInputState
	name        *t.TextInputState
	description *t.TextAreaState
	file        t.Signal[string]
	setup       *t.TextInputState
	onRequest   *t.TextInputState
	onResponse  *t.TextInputState
	follow      *t.CheckboxState
	verifySSL   *t.CheckboxState
	cookies     *t.CheckboxState
	substitute  *t.CheckboxState
	proxy       *t.TextInputState
	timeout     *t.TextInputState

	// title mirrors the name/URL for the tab strip.
	title t.Signal[string]
	// dirty is true when the request differs from what was last loaded/saved.
	dirty t.Signal[bool]

	requestTab  t.Signal[string]
	responseTab t.Signal[string]
	// Tab strips scroll sideways in panels too narrow for all their tabs.
	requestTabView  *tabView
	responseTabView *tabView

	// Exchange state.
	phase    t.Signal[exchangePhase]
	err      t.AnySignal[error]
	trace    t.AnySignal[[]model.TraceEvent]
	response t.AnySignal[*model.Response]
	// fromHistory is set when the response was loaded from history rather than sent.
	fromHistory t.AnySignal[*model.HistoryEntry]
	// responseBody holds the formatted response body for the read-only viewer.
	responseBody       *t.TextAreaState
	responseBodyScroll *t.ScrollState
	responseHeaders    *t.TableState[model.Header]
	responseCookies    *t.TableState[model.Cookie]
	spinner            *t.SpinnerState
	generation         uint64
	cancel             context.CancelFunc

	// syncing suppresses URL<->query feedback loops.
	syncing bool
}

func newSession(id int, req model.Request) *Session {
	s := &Session{
		id:          id,
		method:      t.NewSignal(model.MethodGet),
		url:         t.NewTextInputState(""),
		headers:     newKVEditor(fmt.Sprintf("req-headers-%d", id), false, headerSuggestions()),
		query:       newKVEditor(fmt.Sprintf("req-query-%d", id), false, nil),
		pathParams:  newKVEditor(fmt.Sprintf("req-path-%d", id), true, nil),
		form:        newKVEditor(fmt.Sprintf("req-form-%d", id), false, nil),
		bodyType:    t.NewSignal(model.BodyNone),
		contentType: t.NewSignal("application/json"),
		body:        t.NewTextAreaState(""),
		bodyScroll:  t.NewScrollState(),
		authType:    t.NewSignal(model.AuthNone),
		username:    t.NewTextInputState(""),
		password:    t.NewTextInputState(""),
		token:       t.NewTextInputState(""),
		name:        t.NewTextInputState(""),
		description: t.NewTextAreaState(""),
		file:        t.NewSignal(""),
		setup:       t.NewTextInputState(""),
		onRequest:   t.NewTextInputState(""),
		onResponse:  t.NewTextInputState(""),
		follow:      t.NewCheckboxState(true),
		verifySSL:   t.NewCheckboxState(true),
		cookies:     t.NewCheckboxState(true),
		substitute:  t.NewCheckboxState(true),
		proxy:       t.NewTextInputState(""),
		timeout:     t.NewTextInputState("5.0"),
		title:       t.NewSignal("Untitled"),
		dirty:       t.NewSignal(false),
		requestTab:  t.NewSignal("headers"),
		responseTab: t.NewSignal("body"),

		requestTabView:  newTabView(),
		responseTabView: newTabView(),

		phase:              t.NewSignal(exchangeIdle),
		err:                t.NewAnySignal[error](nil),
		trace:              t.NewAnySignal[[]model.TraceEvent](nil),
		response:           t.NewAnySignal[*model.Response](nil),
		fromHistory:        t.NewAnySignal[*model.HistoryEntry](nil),
		responseBody:       t.NewTextAreaState(""),
		responseBodyScroll: t.NewScrollState(),
		responseHeaders:    t.NewTableState[model.Header](nil),
		responseCookies:    t.NewTableState[model.Cookie](nil),
		spinner:            t.NewSpinnerState(t.SpinnerDots),
	}
	s.responseBody.ReadOnly.Set(true)
	s.query.onChange = s.queryEdited
	s.headers.onChange = s.touch
	s.pathParams.onChange = s.touch
	s.form.onChange = s.touch
	s.Load(req)
	return s
}

// Snapshot returns the request as currently edited.
func (s *Session) Snapshot() model.Request {
	timeout, err := strconv.ParseFloat(strings.TrimSpace(s.timeout.GetText()), 64)
	if err != nil || timeout <= 0 {
		timeout = model.DefaultOptions().TimeoutSeconds
	}
	return model.Request{
		Name:        strings.TrimSpace(s.name.GetText()),
		Description: s.description.GetText(),
		Method:      s.method.Peek(),
		URL:         s.url.GetText(),
		Headers:     s.headers.Values(),
		Query:       s.query.Values(),
		PathParams:  s.pathParams.Values(),
		Body: model.Body{
			Type:        s.bodyType.Peek(),
			Raw:         s.body.GetText(),
			ContentType: s.contentType.Peek(),
			Form:        s.form.Values(),
		},
		Auth: model.Auth{
			Type:     s.authType.Peek(),
			Username: s.username.GetText(),
			Password: s.password.GetText(),
			Token:    s.token.GetText(),
		},
		Options: model.Options{
			FollowRedirects:         s.follow.IsChecked(),
			VerifySSL:               s.verifySSL.IsChecked(),
			AttachCookies:           s.cookies.IsChecked(),
			SubstituteBodyVariables: s.substitute.IsChecked(),
			ProxyURL:                s.proxy.GetText(),
			TimeoutSeconds:          timeout,
		},
		Scripts: model.Scripts{
			Setup:      s.setup.GetText(),
			OnRequest:  s.onRequest.GetText(),
			OnResponse: s.onResponse.GetText(),
		},
		File: s.file.Peek(),
	}
}

// Load replaces the editor contents with req.
func (s *Session) Load(req model.Request) {
	req = req.Clone()
	s.syncing = true
	defer func() { s.syncing = false }()

	if req.Method == "" {
		req.Method = model.MethodGet
	}
	s.method.Set(req.Method)
	s.url.SetText(req.URL)
	s.headers.Load(req.Headers)
	s.query.Load(req.Query)
	if !strings.Contains(req.URL, "?") && len(req.Query) > 0 {
		s.writeQueryToURL()
	}
	s.pathParams.Load(req.PathParams)
	s.syncPathParams()
	s.form.Load(req.Body.Form)
	bodyType := req.Body.Type
	if bodyType == "" {
		bodyType = model.BodyNone
	}
	s.bodyType.Set(bodyType)
	if req.Body.ContentType != "" {
		s.contentType.Set(req.Body.ContentType)
	}
	s.body.SetText(req.Body.Raw)
	authType := req.Auth.Type
	if authType == "" {
		authType = model.AuthNone
	}
	s.authType.Set(authType)
	s.username.SetText(req.Auth.Username)
	s.password.SetText(req.Auth.Password)
	s.token.SetText(req.Auth.Token)
	s.name.SetText(req.Name)
	s.description.SetText(req.Description)
	s.file.Set(req.File)
	s.setup.SetText(req.Scripts.Setup)
	s.onRequest.SetText(req.Scripts.OnRequest)
	s.onResponse.SetText(req.Scripts.OnResponse)
	s.follow.SetChecked(req.Options.FollowRedirects)
	s.verifySSL.SetChecked(req.Options.VerifySSL)
	s.cookies.SetChecked(req.Options.AttachCookies)
	s.substitute.SetChecked(req.Options.SubstituteBodyVariables)
	s.proxy.SetText(req.Options.ProxyURL)
	s.timeout.SetText(strconv.FormatFloat(req.Options.TimeoutSeconds, 'f', -1, 64))
	s.title.Set(req.DisplayName())
	s.dirty.Set(false)
}

// touch marks the session as edited and refreshes its title.
func (s *Session) touch() {
	if s.syncing {
		return
	}
	s.dirty.Set(true)
	name := strings.TrimSpace(s.name.GetText())
	if name == "" {
		name = s.url.GetText()
	}
	if name == "" {
		name = "Untitled"
	}
	s.title.Set(name)
}

// urlEdited keeps the query and path parameter tables in step with the URL.
func (s *Session) urlEdited() {
	if s.syncing {
		return
	}
	s.syncing = true
	s.syncQueryFromURL()
	s.syncPathParams()
	s.syncing = false
	s.touch()
}

// queryEdited rewrites the URL's query string from the query table.
func (s *Session) queryEdited() {
	if s.syncing {
		return
	}
	s.syncing = true
	s.writeQueryToURL()
	s.syncing = false
	s.touch()
}

// writeQueryToURL replaces the URL's query string with the enabled query rows.
func (s *Session) writeQueryToURL() {
	raw := s.url.GetText()
	base, fragment := raw, ""
	if i := strings.IndexByte(base, '#'); i >= 0 {
		base, fragment = base[:i], base[i:]
	}
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	var parts []string
	for _, kv := range s.query.Values() {
		if kv.Enabled {
			parts = append(parts, encodeQueryPart(kv.Name)+"="+encodeQueryPart(kv.Value))
		}
	}
	if len(parts) > 0 {
		base += "?" + strings.Join(parts, "&")
	}
	s.url.SetText(base + fragment)
	s.url.CursorEnd()
}

// encodeQueryPart escapes a query component but leaves ${VAR} references readable.
func encodeQueryPart(s string) string {
	escaped := url.QueryEscape(s)
	for _, pair := range [][2]string{{"%24", "$"}, {"%7B", "{"}, {"%7D", "}"}} {
		escaped = strings.ReplaceAll(escaped, pair[0], pair[1])
	}
	return escaped
}

func (s *Session) syncQueryFromURL() {
	raw := s.url.GetText()
	q := strings.IndexByte(raw, '?')
	var parsed []model.KeyValue
	if q >= 0 {
		query := raw[q+1:]
		if i := strings.IndexByte(query, '#'); i >= 0 {
			query = query[:i]
		}
		for _, part := range strings.Split(query, "&") {
			if part == "" {
				continue
			}
			name, value, _ := strings.Cut(part, "=")
			if decoded, err := url.QueryUnescape(name); err == nil {
				name = decoded
			}
			if decoded, err := url.QueryUnescape(value); err == nil {
				value = decoded
			}
			parsed = append(parsed, model.KeyValue{Name: name, Value: value, Enabled: true})
		}
	}
	// Disabled rows aren't in the URL, so keep them.
	for _, kv := range s.query.Values() {
		if !kv.Enabled {
			parsed = append(parsed, kv)
		}
	}
	if !sameKVs(parsed, s.query.Values()) {
		s.query.Load(parsed)
	}
}

// syncPathParams makes the path parameter rows match the :params in the URL,
// keeping values for names that still exist.
func (s *Session) syncPathParams() {
	names := model.PathParamNames(s.url.GetText())
	existing := map[string]string{}
	for _, kv := range s.pathParams.Values() {
		existing[kv.Name] = kv.Value
	}
	next := make([]model.KeyValue, 0, len(names))
	for _, name := range names {
		next = append(next, model.KeyValue{Name: name, Value: existing[name], Enabled: true})
	}
	if !sameKVs(next, s.pathParams.Values()) {
		s.pathParams.Load(next)
	}
}

func sameKVs(a, b []model.KeyValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Send starts an exchange using sender. Any in-flight exchange is cancelled.
// onDone runs on the UI goroutine when a response arrives.
func (s *Session) Send(sender client.Sender, variables map[string]string, onDone func(model.Request, *model.Response)) {
	s.Cancel()
	req := s.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	s.generation++
	generation := s.generation
	s.cancel = cancel
	s.phase.Set(exchangeSending)
	s.err.Set(nil)
	s.trace.Set(nil)
	s.spinner.Start()

	call := client.Call{
		Request:   req,
		Variables: variables,
		OnTrace: func(event model.TraceEvent) {
			t.Dispatch(func() {
				if generation == s.generation {
					s.trace.Set(mergeTrace(s.trace.Peek(), event))
				}
			})
		},
	}
	go func() {
		resp, err := sender.Send(ctx, call)
		t.Dispatch(func() {
			if generation != s.generation {
				return
			}
			cancelled := ctx.Err() != nil
			s.cancel = nil
			cancel()
			s.spinner.Stop()
			switch {
			case cancelled:
				s.phase.Set(exchangeCancelled)
			case err != nil:
				s.err.Set(err)
				s.phase.Set(exchangeFailed)
			default:
				s.showResponse(resp, nil)
				s.phase.Set(exchangeDone)
				if onDone != nil {
					onDone(req, resp)
				}
			}
		})
	}()
}

// Cancel abandons the in-flight exchange, if any.
func (s *Session) Cancel() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
	s.generation++
	s.spinner.Stop()
	s.phase.Set(exchangeCancelled)
}

// showResponse displays resp. entry is non-nil when it came from history.
func (s *Session) showResponse(resp *model.Response, entry *model.HistoryEntry) {
	s.response.Set(resp)
	s.fromHistory.Set(entry)
	if resp == nil {
		return
	}
	s.trace.Set(resp.Trace)
	s.responseBody.SetText(formatBody(resp))
	s.responseBody.CursorIndex.Set(0)
	s.responseBodyScroll.SetOffset(0)
	s.responseHeaders.SetRows(resp.Headers)
	s.responseCookies.SetRows(resp.Cookies)
}

// mergeTrace replaces the event for the same stage or appends it.
func mergeTrace(events []model.TraceEvent, event model.TraceEvent) []model.TraceEvent {
	out := append([]model.TraceEvent(nil), events...)
	for i := range out {
		if out[i].Stage == event.Stage {
			out[i] = event
			return out
		}
	}
	return append(out, event)
}

// historyEntry builds a history record for a completed exchange.
func historyEntry(id int64, req model.Request, resp *model.Response) model.HistoryEntry {
	return model.HistoryEntry{ID: id, Request: req.Clone(), Response: resp, SentAt: time.Now()}
}
