package ui

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/model"
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

// streamPhase is where a gRPC stream opened with ctrl+o is up to. Its call
// keeps the sending side open until the user ends it.
type streamPhase int

const (
	streamNone streamPhase = iota
	// streamStarting is before the call starts, when it can't take messages.
	streamStarting
	// streamOpen takes messages: sending puts the editor's message on it.
	streamOpen
	// streamEnded takes no more messages, because the user ended it or the
	// method's client doesn't stream, and waits for the server to end the
	// call.
	streamEnded
)

// Session is one open request tab: the editor state for the request plus the
// state of its most recent exchange. All mutation happens on the UI goroutine.
//
// Snapshot and Load are the only bridge between the editor widgets and
// model.Request, so the rest of the app (and any future HTTP client) only
// ever deals in plain model values.
type Session struct {
	id int

	// Request editing state. kind is the kind of request being edited;
	// method is only used while that is HTTP.
	kind              t.Signal[model.KindID]
	method            t.Signal[model.Method]
	url               *t.TextInputState
	headers           *kvEditor
	query             *kvEditor
	pathParams        *kvEditor
	form              *kvEditor
	bodyType          t.Signal[model.BodyType]
	contentType       t.Signal[string]
	body              *t.TextAreaState
	bodyScroll        *t.ScrollState
	descriptionScroll *t.ScrollState
	authType          t.Signal[model.AuthType]
	username          *t.TextInputState
	password          *t.TextInputState
	token             *t.TextInputState
	name              *t.TextInputState
	description       *t.TextAreaState
	file              t.Signal[string]
	// scripts aren't edited in Posting 3, but are kept so saving a
	// Posting 2 request doesn't drop them.
	scripts       model.Scripts
	variableScope *model.VariableScope
	// payloads edit the payload of each kind that has one, for the
	// session's whole life, so switching kinds loses nothing.
	payloads   map[model.KindID]payloadEditor
	follow     *t.CheckboxState
	verifySSL  *t.CheckboxState
	cookies    *t.CheckboxState
	substitute *t.CheckboxState
	proxy      *t.TextInputState
	timeout    *t.TextInputState

	// Variable completion for the fields that aren't key/value rows.
	urlVars, usernameVars, passwordVars, tokenVars, proxyVars, bodyVars *completion

	// title mirrors the name/URL for the tab strip.
	title t.Signal[string]
	// dirty is true when the request differs from what was last loaded/saved.
	dirty t.Signal[bool]
	// preview marks the preview tab: the one tab that requests opened from
	// the sidebar load into, replacing each other, until the user commits to
	// it by editing, sending or saving it or asking to keep it (see
	// App.openRequest). A preview tab never has unsaved changes.
	preview t.Signal[bool]

	requestTab  t.Signal[string]
	responseTab t.Signal[string]
	// Tab strips scroll sideways in panels too narrow for all their tabs.
	requestTabView  *tabView
	responseTabView *tabView
	// Where tabs too long for a short panel are scrolled to.
	authScroll    *formScroll
	optionsScroll *formScroll
	traceScroll   *t.ScrollState

	// Exchange state.
	phase    t.Signal[exchangePhase]
	err      t.AnySignal[error]
	trace    t.AnySignal[[]model.TraceEvent]
	response t.AnySignal[*model.Response]
	// sent is the request the shown response answers, and responseStatus
	// how the response reads for it (see model.StatusOf). The status is read
	// once, when the response is shown, since a GraphQL status parses the
	// whole body.
	sent           model.Request
	responseStatus model.Status
	// fromHistory is set when the response was loaded from history rather than sent.
	fromHistory t.AnySignal[*model.HistoryEntry]
	// responseBody holds the formatted response body for the read-only viewer.
	responseBody               *t.TextAreaState
	responseBodyScroll         *t.ScrollState
	responseBodyViewportHeight int
	// responseVisual is the body viewer's visual mode, where moving selects.
	responseVisual        t.Signal[bool]
	responseHeaders       *t.TableState[model.Header]
	responseHeadersScroll *t.ScrollState
	responseCookies       *t.TableState[model.Cookie]
	responseCookiesScroll *t.ScrollState
	// responseTrailers are a gRPC response's trailers.
	responseTrailers       *t.TableState[model.Header]
	responseTrailersScroll *t.ScrollState
	spinner                *t.SpinnerState
	generation             uint64
	cancel                 context.CancelFunc
	// live is set once the exchange in flight has shown part of its response.
	live bool
	// stream is the open stream of the exchange in flight, if it has one.
	stream    *client.Stream
	streaming t.Signal[streamPhase]
	// dispatch schedules exchange updates on the UI goroutine.
	dispatch func(func())

	// prettifyJSON indents JSON response bodies.
	prettifyJSON bool

	// syncing suppresses URL<->query feedback loops.
	syncing bool
}

func newSession(id int, req model.Request) *Session {
	s := &Session{
		id:                id,
		kind:              t.NewSignal(model.KindHTTP),
		method:            t.NewSignal(model.MethodGet),
		url:               t.NewTextInputState(""),
		headers:           newKVEditor(fmt.Sprintf("req-headers-%d", id), false, headerSuggestions()),
		query:             newKVEditor(fmt.Sprintf("req-query-%d", id), false, nil),
		pathParams:        newKVEditor(fmt.Sprintf("req-path-%d", id), true, nil),
		form:              newKVEditor(fmt.Sprintf("req-form-%d", id), false, nil),
		bodyType:          t.NewSignal(model.BodyNone),
		contentType:       t.NewSignal("application/json"),
		body:              t.NewTextAreaState(""),
		bodyScroll:        t.NewScrollState(),
		descriptionScroll: t.NewScrollState(),
		authType:          t.NewSignal(model.AuthNone),
		username:          t.NewTextInputState(""),
		password:          t.NewTextInputState(""),
		token:             t.NewTextInputState(""),
		name:              t.NewTextInputState(""),
		description:       t.NewTextAreaState(""),
		file:              t.NewSignal(""),
		follow:            t.NewCheckboxState(true),
		verifySSL:         t.NewCheckboxState(true),
		cookies:           t.NewCheckboxState(true),
		substitute:        t.NewCheckboxState(true),
		proxy:             t.NewTextInputState(""),
		timeout:           t.NewTextInputState("5.0"),
		title:             t.NewSignal("Untitled"),
		dirty:             t.NewSignal(false),
		preview:           t.NewSignal(false),
		requestTab:        t.NewSignal("headers"),
		responseTab:       t.NewSignal("body"),

		requestTabView:  newTabView(),
		responseTabView: newTabView(),

		authScroll:    newFormScroll(),
		optionsScroll: newFormScroll(),
		traceScroll:   t.NewScrollState(),

		phase:                  t.NewSignal(exchangeIdle),
		streaming:              t.NewSignal(streamNone),
		err:                    t.NewAnySignal[error](nil),
		trace:                  t.NewAnySignal[[]model.TraceEvent](nil),
		response:               t.NewAnySignal[*model.Response](nil),
		fromHistory:            t.NewAnySignal[*model.HistoryEntry](nil),
		responseBody:           t.NewTextAreaState(""),
		responseBodyScroll:     t.NewScrollState(),
		responseVisual:         t.NewSignal(false),
		responseHeaders:        t.NewTableState[model.Header](nil),
		responseHeadersScroll:  t.NewScrollState(),
		responseCookies:        t.NewTableState[model.Cookie](nil),
		responseCookiesScroll:  t.NewScrollState(),
		responseTrailers:       t.NewTableState[model.Header](nil),
		responseTrailersScroll: t.NewScrollState(),
		spinner:                t.NewSpinnerState(t.SpinnerDots),
		dispatch:               t.Dispatch,
	}
	s.urlVars, s.usernameVars, s.passwordVars = newCompletion(), newCompletion(), newCompletion()
	s.tokenVars, s.proxyVars, s.bodyVars = newCompletion(), newCompletion(), newCompletion()
	s.responseBody.ReadOnly.Set(true)
	s.query.onChange = s.queryEdited
	s.headers.onChange = s.touch
	s.pathParams.onChange = s.touch
	s.form.onChange = s.touch
	toTabs := func() { t.RequestFocus(requestTabsID) }
	s.headers.onLeaveTop, s.query.onLeaveTop, s.pathParams.onLeaveTop = toTabs, toTabs, toTabs
	s.form.onLeaveTop = func() { t.RequestFocus("req-body-type") }
	s.payloads = map[model.KindID]payloadEditor{}
	for id, view := range kindViews {
		if view.newEditor != nil {
			s.payloads[id] = view.newEditor(s)
		}
	}
	s.Load(req)
	return s
}

// Snapshot returns the request as currently edited.
func (s *Session) Snapshot() model.Request {
	timeout, err := strconv.ParseFloat(strings.TrimSpace(s.timeout.GetText()), 64)
	if err != nil || timeout <= 0 {
		timeout = model.DefaultOptions().TimeoutSeconds
	}
	req := model.Request{
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
		Scripts:       s.scripts,
		VariableScope: s.variableScope.Clone(),
		File:          s.file.Peek(),
	}
	if e := s.payloads[s.kind.Peek()]; e != nil {
		req.Payload = e.payload()
	}
	return model.Normalize(req)
}

// Load replaces the editor contents with req.
func (s *Session) Load(req model.Request) {
	req = req.Clone()
	s.syncing = true
	defer func() { s.syncing = false }()

	if req.Method == "" {
		req.Method = model.MethodGet
	}
	s.setKind(req.Kind().ID)
	for _, e := range s.payloads {
		e.load(req)
	}
	s.method.Set(req.Method)
	s.headers.Load(req.Headers)
	s.query.Load(req.Query)
	base, _, _ := strings.Cut(req.URL, "#")
	_, query, _ := strings.Cut(base, "?")
	// An empty URL query falls back to saved rows, as model.Resolve does.
	if query == "" && len(req.Query) > 0 {
		s.url.SetText(s.urlWithQuery(req.URL))
		s.url.CursorEnd()
	} else {
		s.url.SetText(req.URL)
	}
	if query != "" {
		s.syncQueryFromURL()
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
	s.scripts = req.Scripts
	s.variableScope = req.VariableScope.Clone()
	s.follow.SetChecked(req.Options.FollowRedirects)
	s.verifySSL.SetChecked(req.Options.VerifySSL)
	s.cookies.SetChecked(req.Options.AttachCookies)
	s.substitute.SetChecked(req.Options.SubstituteBodyVariables)
	s.proxy.SetText(req.Options.ProxyURL)
	s.timeout.SetText(strconv.FormatFloat(req.Options.TimeoutSeconds, 'f', -1, 64))
	s.title.Set(req.DisplayName())
	s.dirty.Set(false)
	// Show a newly loaded body from its start. (Unfocused inputs already
	// show the start of their text.)
	s.body.ClearSelection()
	s.description.ClearSelection()
	s.body.CursorIndex.Set(0)
	s.description.CursorIndex.Set(0)
	s.bodyScroll.SetOffset(0)
	s.descriptionScroll.SetOffset(0)
}

// setKind changes the kind of request the session edits. The kind's first
// tab is selected, so its own editor is what shows. The auth type becomes
// the one Snapshot sends, so auth the kind can't send, such as digest for
// gRPC, isn't left showing.
func (s *Session) setKind(id model.KindID) {
	if s.kind.Peek() == id {
		return
	}
	s.kind.Set(id)
	s.authType.Set(s.Snapshot().Auth.Type)
	s.requestTab.Set(s.requestTabList()[0].key)
}

// touch marks the session as edited and refreshes its title.
func (s *Session) touch() {
	if s.syncing {
		return
	}
	s.markEdited()
	titled := model.Request{Name: strings.TrimSpace(s.name.GetText()), URL: s.url.GetText()}
	if e := s.payloads[s.kind.Peek()]; e != nil {
		titled.Payload = e.payload()
	}
	s.title.Set(titled.DisplayName())
}

// markEdited records that the request has unsaved changes. An edited tab is
// kept: it stops being the preview, so opening another request can't replace
// it and lose the changes.
func (s *Session) markEdited() {
	s.dirty.Set(true)
	s.preview.Set(false)
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

// writeQueryToURL replaces the URL's query string with the enabled query
// rows, as an edit the user can undo.
func (s *Session) writeQueryToURL() {
	replaceToEnd(s.url, s.urlWithQuery(s.url.GetText()))
}

// urlWithQuery is raw with its query string replaced by the enabled query rows.
func (s *Session) urlWithQuery(raw string) string {
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
	return base + fragment
}

// replaceToEnd replaces input's text as one undoable edit and puts the cursor
// at the end. ReplaceText clamps the cursor, and a byte count is never less
// than the grapheme count.
func replaceToEnd(input *t.TextInputState, text string) {
	input.ReplaceText(text, len(text))
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
	// A question mark inside the fragment is not a query delimiter.
	raw, _, _ := strings.Cut(s.url.GetText(), "#")
	q := strings.IndexByte(raw, '?')
	var parsed []model.KeyValue
	if q >= 0 {
		query := raw[q+1:]
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
// A gRPC call opens stream when it isn't nil. The response shows as it
// arrives, and onDone runs on the UI goroutine once it has all arrived.
func (s *Session) Send(sender client.Sender, variables map[string]string, stream *client.Stream, onDone func(model.Request, *model.Response, model.Status)) {
	s.Cancel()
	req := s.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	s.generation++
	generation := s.generation
	s.cancel = cancel
	s.live = false
	s.stream = stream
	s.phase.Set(exchangeSending)
	s.err.Set(nil)
	s.trace.Set(nil)
	s.spinner.Start()
	inFlight := func() bool { return generation == s.generation && s.phase.Peek() == exchangeSending }

	call := client.Call{
		Request:   req,
		Variables: variables,
		OnTrace: func(event model.TraceEvent) {
			s.dispatch(func() {
				if generation == s.generation {
					s.trace.Set(mergeTrace(s.trace.Peek(), event))
				}
			})
		},
		OnUpdate: func(resp *model.Response) {
			s.dispatch(func() {
				if inFlight() {
					s.showLive(req, resp)
				}
			})
		},
		Stream: stream,
	}
	if stream != nil {
		s.streaming.Set(streamStarting)
		go func() {
			select {
			case <-stream.Ready():
			case <-ctx.Done():
				return
			}
			s.dispatch(func() {
				if inFlight() && s.streaming.Peek() == streamStarting {
					s.streaming.Set(streamEnded)
					if stream.TakesMessages() {
						s.streaming.Set(streamOpen)
					}
				}
			})
		}()
	}
	go func() {
		resp, err := sender.Send(ctx, call)
		s.dispatch(func() {
			if generation != s.generation {
				return
			}
			cancelled := ctx.Err() != nil
			s.cancel = nil
			cancel()
			s.endStream()
			s.spinner.Stop()
			switch {
			// A call the server answered is a response even once stopped.
			case err != nil && cancelled:
				s.phase.Set(exchangeCancelled)
			case err != nil:
				s.err.Set(err)
				s.phase.Set(exchangeFailed)
			default:
				s.sent = req
				s.showResponse(resp, nil)
				s.phase.Set(exchangeDone)
				if onDone != nil {
					onDone(req, resp, s.responseStatus)
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
	s.endStream()
	s.spinner.Stop()
	s.phase.Set(exchangeCancelled)
}

// Stop cancels the in-flight exchange but, unlike Cancel, still shows what
// it returns, so stopping a stream keeps the messages that arrived.
func (s *Session) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Session) endStream() {
	s.stream = nil
	s.streaming.Set(streamNone)
}

// showLive shows the response so far of the exchange in flight, sent as
// req. After the first, an update keeps the body's cursor and scroll, so
// reading earlier messages isn't interrupted.
func (s *Session) showLive(req model.Request, resp *model.Response) {
	if !s.live {
		s.live = true
		s.sent = req
		s.showResponse(resp, nil)
		return
	}
	s.response.Set(resp)
	s.responseBody.SetText(formatBody(resp, s.prettifyJSON))
	s.responseHeaders.SetRows(resp.Headers)
}

// showResponse displays resp. entry is non-nil when it came from history.
func (s *Session) showResponse(resp *model.Response, entry *model.HistoryEntry) {
	if entry != nil {
		s.sent, s.responseStatus = entry.Request, entry.Status
	} else {
		s.responseStatus = model.StatusOf(s.sent, resp)
	}
	s.response.Set(resp)
	s.fromHistory.Set(entry)
	if resp == nil {
		return
	}
	s.trace.Set(resp.Trace)
	s.responseBody.SetText(formatBody(resp, s.prettifyJSON))
	s.responseBody.CursorIndex.Set(0)
	s.responseBodyScroll.SetOffset(0)
	s.responseBody.SelectionAnchor.Set(-1)
	s.responseVisual.Set(false)
	s.responseHeaders.SetRows(resp.Headers)
	s.responseHeadersScroll.SetOffset(0)
	s.responseCookies.SetRows(resp.Cookies)
	s.responseCookiesScroll.SetOffset(0)
	s.responseTrailers.SetRows(resp.Trailers)
	s.responseTrailersScroll.SetOffset(0)
	if !slices.Contains(responseTabKeys(s.sent), s.responseTab.Peek()) {
		s.responseTab.Set("body")
	}
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

// historyEntry builds a history record for a completed exchange whose
// response reads as status.
func historyEntry(id int64, req model.Request, resp *model.Response, status model.Status) model.HistoryEntry {
	return model.HistoryEntry{ID: id, Request: req.Clone(), Response: resp, SentAt: time.Now(), Status: status}
}
