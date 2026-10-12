package client

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// Stream is the sending side of a gRPC call that stays open, so the UI can
// send more messages while the call runs. Its methods are safe to call from
// any goroutine.
type Stream struct {
	mu sync.Mutex
	// parse reads message text for the call's method. It is nil until the
	// call has its schema.
	parse func(text string) ([]proto.Message, error)
	// refusal is why a call whose client doesn't stream takes nothing more.
	refusal error
	queue   []proto.Message
	closed  bool
	ended   bool
	wake    chan struct{}
	ready   chan struct{}
}

// NewStream returns a stream for one call, to pass in its Call.
func NewStream() *Stream {
	return &Stream{wake: make(chan struct{}, 1), ready: make(chan struct{})}
}

// Ready is closed once the call has started and Send takes messages.
func (s *Stream) Ready() <-chan struct{} { return s.ready }

// Send queues text's messages, written as in the request's message: a JSON
// object, or for a client-streaming or bidi method an array of them. It sends
// nothing and returns an error when the call hasn't started or is over, when
// the stream was closed, or when text isn't what the method takes.
func (s *Stream) Send(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.ended:
		return errors.New("the call is over")
	case s.closed:
		return errors.New("the stream was ended, so it takes no more messages")
	case s.parse == nil:
		return errors.New("the call hasn't started yet")
	case s.refusal != nil:
		return s.refusal
	}
	messages, err := s.parse(text)
	if err != nil {
		return err
	}
	s.queue = append(s.queue, messages...)
	s.signal()
	return nil
}

// Close ends the sending side once the queued messages are sent.
func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.signal()
}

// TakesMessages reports whether Send would take a message now: the call has
// started, its client streams, and neither side has ended it.
func (s *Stream) TakesMessages() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parse != nil && s.refusal == nil && !s.closed && !s.ended
}

// start lets the stream take messages, read by parse, or refuse each with
// refusal when it isn't nil.
func (s *Stream) start(parse func(text string) ([]proto.Message, error), refusal error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parse, s.refusal = parse, refusal
	close(s.ready)
}

// end refuses further messages, and stops a next that is waiting.
func (s *Stream) end() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ended = true
	s.signal()
}

// next waits for queued messages. It returns none once the stream is closed
// with nothing left to send, the call is over, or ctx is done.
func (s *Stream) next(ctx context.Context) []proto.Message {
	for {
		s.mu.Lock()
		queued, stop := s.queue, s.ended || s.closed
		s.queue = nil
		s.mu.Unlock()
		if len(queued) > 0 || stop {
			return queued
		}
		select {
		case <-s.wake:
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Stream) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// updateInterval is the least time between a call's progress reports, so a
// fast stream doesn't rebuild its body for every message.
const updateInterval = 100 * time.Millisecond

// progress reports a call's messages as they arrive: at once when the last
// report was long enough ago, otherwise when updateInterval has passed.
type progress struct {
	report    func(header metadata.MD, messages [][]byte, events []model.MessageEvent)
	streaming bool

	mu       sync.Mutex
	header   metadata.MD
	messages [][]byte
	events   []model.MessageEvent
	logBytes int
	limited  bool
	last     time.Time
	pending  bool
	stopped  bool
}

// Keep long-lived streams' logs bounded, including outgoing-only streams
// and empty messages. Reaching this limit stops recording, not the call.
const maxLogEvents = 10_000

func (p *progress) event(kind model.MessageEventKind, title string, body []byte) {
	if p == nil || !p.streaming {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.record(kind, title, body)
	p.schedule()
}

// record appends under mu. Entries and their bodies are never changed, so
// snapshots can safely share their contents with history and the UI.
func (p *progress) record(kind model.MessageEventKind, title string, body []byte) {
	if !p.streaming || p.limited {
		return
	}
	if len(p.events) >= maxLogEvents || p.logBytes+len(body) > maxBodyBytes {
		p.events = append(p.events, model.MessageEvent{At: time.Now(), Kind: model.MessageInfo, Title: "Log limit reached; further messages are not recorded"})
		p.limited = true
		return
	}
	e := model.MessageEvent{At: time.Now(), Kind: kind, Title: title, Body: body}
	if len(body) > 0 {
		e.ContentType = "application/json"
	}
	p.events = append(p.events, e)
	p.logBytes += len(body)
}

// add records a message, as JSON, and the metadata the call began with.
func (p *progress) add(header metadata.MD, message []byte) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.header = header
	p.messages = append(p.messages, message)
	p.record(model.MessageReceived, "", message)
	p.schedule()
}

func (p *progress) schedule() {
	if p.report == nil || p.pending || p.stopped {
		return
	}
	p.pending = true
	time.AfterFunc(max(time.Until(p.last.Add(updateInterval)), 0), p.flush)
}

func (p *progress) flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.pending, p.last = false, time.Now()
	// Serialise callbacks with each other and stop, so an older snapshot
	// cannot arrive after a newer one or the completed response.
	p.report(p.header, slices.Clip(p.messages), slices.Clip(p.events))
}

// stop ends the reports; the call's response follows.
func (p *progress) stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
}

// finish retains the terminal event even when the message log filled up.
func (p *progress) finish(code int, message string, body []byte) []model.MessageEvent {
	if p == nil || !p.streaming {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	kind := model.MessageInfo
	if code != 0 {
		kind = model.MessageError
	}
	title := "Call ended: " + model.GRPCCodeName(code)
	if message != "" {
		title += " · " + message
	}
	p.events = append(p.events, model.MessageEvent{At: time.Now(), Kind: kind, Title: title, Body: body, ContentType: "application/json"})
	return slices.Clip(p.events)
}
