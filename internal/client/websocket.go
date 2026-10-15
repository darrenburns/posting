package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/gorilla/websocket"
)

const maxWebSocketMessageBytes = 1 << 20

// WebSocket shares HTTP's TLS settings, OAuth provider and cookie jar, but
// owns a separate, full-duplex connection. It never redirects or reconnects.
type WebSocket struct{ HTTP *HTTP }

type socketMessage struct {
	typ  int
	body []byte
}

// Socket queues interactive messages without blocking the UI on network I/O.
// A Socket belongs to one connection and cannot be reopened.
type Socket struct {
	mu     sync.Mutex
	parse  func(model.WebSocket) (socketMessage, error)
	ended  bool
	queue  chan socketMessage
	ready  chan struct{}
	closed chan struct{}
}

func NewSocket() *Socket {
	return &Socket{queue: make(chan socketMessage, 32), ready: make(chan struct{}), closed: make(chan struct{})}
}

func (s *Socket) Ready() <-chan struct{} { return s.ready }

func (s *Socket) TakesMessages() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parse != nil && !s.ended
}

func (s *Socket) Send(message model.WebSocket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended || s.parse == nil {
		return errors.New("the WebSocket is not connected")
	}
	m, err := s.parse(message)
	if err != nil {
		return err
	}
	select {
	case s.queue <- m:
		return nil
	default:
		return errors.New("the WebSocket send queue is full; wait before sending again")
	}
}

// Close requests a normal close handshake. Unsent queued messages are discarded.
func (s *Socket) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ended {
		s.ended = true
		close(s.closed)
	}
}

func (s *Socket) start(call Call) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parse = func(message model.WebSocket) (socketMessage, error) {
		if call.Request.Options.SubstituteBodyVariables {
			if call.ResolveBody != nil {
				text, err := call.ResolveBody(message.Message)
				if err != nil {
					return socketMessage{}, err
				}
				message.Message = text
			} else {
				message.Message = model.Substitute(message.Message, call.Lookup)
			}
		}
		return encodeSocketMessage(message)
	}
	close(s.ready)
}

func encodeSocketMessage(message model.WebSocket) (socketMessage, error) {
	// Bound the encoded input before allocating its decoded bytes, too.
	if len(message.Message) > base64.StdEncoding.EncodedLen(maxWebSocketMessageBytes) {
		return socketMessage{}, errors.New("WebSocket messages are limited to 1 MiB")
	}
	m := socketMessage{typ: websocket.TextMessage, body: []byte(message.Message)}
	if message.Binary {
		var err error
		m.typ = websocket.BinaryMessage
		m.body, err = base64.StdEncoding.DecodeString(message.Message)
		if err != nil {
			return m, fmt.Errorf("binary messages must be base64: %w", err)
		}
	} else if !utf8.Valid(m.body) {
		return m, errors.New("text messages must be valid UTF-8")
	}
	if len(m.body) > maxWebSocketMessageBytes {
		return m, errors.New("WebSocket messages are limited to 1 MiB")
	}
	return m, nil
}

func socketEvent(kind model.MessageEventKind, m socketMessage) model.MessageEvent {
	e := model.MessageEvent{At: time.Now(), Kind: kind, Body: m.body, FrameType: "text", ContentType: "text/plain"}
	if m.typ == websocket.BinaryMessage {
		e.FrameType, e.ContentType = "binary", "application/octet-stream"
	} else if json.Valid(m.body) {
		e.ContentType = "application/json"
	}
	return e
}

func (w *WebSocket) Send(ctx context.Context, call Call) (*model.Response, error) {
	if call.Socket != nil {
		defer call.Socket.Close()
	}
	req, err := model.Resolve(call.Request, call.Lookup)
	if err != nil {
		return nil, err
	}
	message, ok := req.Payload.(model.WebSocket)
	if !ok {
		return nil, errors.New("not a WebSocket request")
	}
	target, err := url.Parse(req.URL)
	if err != nil || target.Host == "" || target.Fragment != "" || (target.Scheme != "ws" && target.Scheme != "wss") {
		return nil, errors.New("enter a ws:// or wss:// URL with a host and no fragment")
	}
	h := w.HTTP
	h.init()
	if h.tlsErr != nil {
		return nil, h.tlsErr
	}
	req.Auth, err = h.OAuth.authorize(ctx, req, true, call.OnAuth)
	if err != nil {
		return nil, err
	}
	transport, err := h.transport(req)
	if err != nil {
		return nil, err
	}
	defer transport.CloseIdleConnections()
	timeout := req.Options.TimeoutSeconds
	if timeout <= 0 {
		timeout = model.DefaultOptions().TimeoutSeconds
	}
	budget := time.Duration(timeout * float64(time.Second))
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(budget, func() { cancel(context.DeadlineExceeded) })
	defer timer.Stop()
	// Gorilla bounds upgrade I/O with a deadline, but doesn't interrupt a
	// stalled upgrade on cancellation. Closing the raw socket does both, and
	// also interrupts blocked reads/writes after the upgrade succeeds.
	stopCancel := func() bool { return false }
	defer func() { stopCancel() }()
	dialer := websocket.Dialer{
		TLSClientConfig: transport.TLSClientConfig, Proxy: transport.Proxy,
		HandshakeTimeout: budget,
		// Use the lifetime context, not the temporary handshake context.
		NetDialContext: func(_ context.Context, network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err == nil {
				stopCancel = context.AfterFunc(ctx, func() { conn.Close() })
			}
			return conn, err
		},
	}
	if req.Options.AttachCookies {
		dialer.Jar = h.jar
	}
	headersIn := make(http.Header)
	for _, header := range req.Headers {
		if header.Enabled && header.Name != "" {
			headersIn.Add(header.Name, header.Value)
		}
	}
	if headersIn.Get("User-Agent") == "" && h.UserAgent != "" {
		headersIn.Set("User-Agent", h.UserAgent)
	}
	switch req.Auth.Type {
	case model.AuthBasic:
		headersIn.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(req.Auth.Username+":"+req.Auth.Password)))
	case model.AuthBearer:
		headersIn.Set("Authorization", "Bearer "+req.Auth.Token)
	}
	trace := newTracer(call.OnTrace, target.Scheme == "wss")
	started := time.Now()
	trace.start(model.TraceConnect)
	conn, resp, err := dialer.DialContext(ctx, req.URL, headersIn)
	if err != nil {
		trace.fail()
		if resp == nil {
			return nil, describeError(ctx, err, timeout)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxWebSocketMessageBytes))
		return &model.Response{StatusCode: resp.StatusCode, Reason: reason(resp), Proto: resp.Proto,
			Headers: headers(resp.Header), Body: body, URL: req.URL, Method: model.MethodGet,
			StreamError: err.Error(), Trace: trace.events(), Elapsed: time.Since(started), ReceivedAt: time.Now()}, nil
	}
	defer conn.Close()
	trace.complete(model.TraceConnect)
	trace.start(model.TraceReceiveBody)
	conn.SetReadLimit(maxWebSocketMessageBytes)
	if call.Socket != nil {
		timer.Stop()
		call.Socket.start(call)
	}
	out := &model.Response{StatusCode: resp.StatusCode, Reason: reason(resp), Proto: "WebSocket",
		Headers: headers(resp.Header), Cookies: cookies(resp.Cookies()), URL: req.URL, Method: model.MethodGet,
		Events: []model.MessageEvent{{At: time.Now(), Kind: model.MessageInfo, Title: "WebSocket connected"}}}
	report := func() {
		if call.OnUpdate != nil {
			snapshot := *out
			snapshot.Events = slices.Clip(out.Events)
			snapshot.Elapsed = time.Since(started)
			snapshot.Trace = trace.events()
			call.OnUpdate(&snapshot)
		}
	}
	report()

	// One reader processes control frames even while idle (the library
	// replies to ping/close). Only this goroutine below owns the log/writer.
	incoming := make(chan socketMessage)
	readDone := make(chan error, 1)
	readerCtx, stopReader := context.WithCancel(ctx)
	defer stopReader()
	go func() {
		for {
			typ, body, err := conn.ReadMessage()
			if err != nil {
				readDone <- err
				return
			}
			if typ == websocket.TextMessage && !utf8.Valid(body) {
				_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid UTF-8"), time.Now().Add(time.Second))
				readDone <- errors.New("received invalid UTF-8 in a text message")
				return
			}
			select {
			case incoming <- socketMessage{typ, body}:
			case <-readerCtx.Done():
				return
			}
		}
	}()
	logBytes := 0
	record := func(kind model.MessageEventKind, m socketMessage) error {
		if len(out.Events) >= maxLogEvents || logBytes+len(m.body) > maxBodyBytes {
			return errors.New("WebSocket log limit reached (10,000 messages or 64 MiB)")
		}
		out.Events = append(out.Events, socketEvent(kind, m))
		logBytes += len(m.body)
		return nil
	}
	write := func(m socketMessage) error {
		conn.SetWriteDeadline(time.Now().Add(budget))
		if err := conn.WriteMessage(m.typ, m.body); err != nil {
			return err
		}
		return record(model.MessageSent, m)
	}
	var outgoing <-chan socketMessage
	var disconnect <-chan struct{}
	if call.Socket != nil {
		outgoing, disconnect = call.Socket.queue, call.Socket.closed
	} else if message.Message != "" {
		var m socketMessage
		m, err = encodeSocketMessage(message)
		if err == nil {
			err = write(m)
		}
	}
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	dirty, localClose := len(out.Events) > 1, false
	for err == nil {
		select {
		case m := <-incoming:
			err = record(model.MessageReceived, m)
			dirty = true
		case m := <-outgoing:
			// A queued send and Close can both be ready. Never start another
			// write once the caller has requested disconnect.
			if !call.Socket.TakesMessages() {
				continue
			}
			err = write(m)
			dirty = true
		case err = <-readDone:
		case <-ctx.Done():
			err = context.Cause(ctx)
		case <-disconnect:
			localClose = true
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			// Bound the close handshake even when the peer doesn't reply.
			conn.NetConn().SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			disconnect, outgoing = nil, nil
		case <-ticker.C:
			if dirty {
				report()
				dirty = false
			}
		}
	}
	closeStatus := &model.WebSocketStatus{Code: websocket.CloseAbnormalClosure}
	var closeErr *websocket.CloseError
	switch {
	case localClose:
		closeStatus.Code = websocket.CloseNormalClosure
	case ctx.Err() != nil:
		out.StreamError = describeError(ctx, err, timeout).Error()
	case errors.As(err, &closeErr):
		closeStatus.Code, closeStatus.Reason = closeErr.Code, closeErr.Text
		if closeErr.Code != websocket.CloseNormalClosure && closeErr.Code != websocket.CloseGoingAway {
			out.StreamError = err.Error()
		}
	default:
		out.StreamError = describeError(ctx, err, timeout).Error()
	}
	out.WebSocket = closeStatus
	title, kind := fmt.Sprintf("WebSocket disconnected: %d", closeStatus.Code), model.MessageInfo
	if closeStatus.Reason != "" {
		title += " · " + closeStatus.Reason
	}
	if out.StreamError != "" {
		title, kind = "WebSocket interrupted: "+out.StreamError, model.MessageError
		trace.fail()
	} else {
		trace.complete(model.TraceReceiveBody)
	}
	out.Events = append(out.Events, model.MessageEvent{At: time.Now(), Kind: kind, Title: title})
	trace.start(model.TraceClosed)
	conn.Close()
	trace.complete(model.TraceClosed)
	out.Elapsed, out.ReceivedAt, out.Trace = time.Since(started), time.Now(), trace.events()
	// A portable transcript for headless output/export; event bodies are
	// base64 in JSON so binary and text payloads both round-trip losslessly.
	out.Body, _ = json.Marshal(out.Events)
	out.BodyContentType = "application/json"
	return out, nil
}
