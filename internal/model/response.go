package model

import (
	"fmt"
	"strings"
	"time"
)

// Header is a single response header. Order and duplicates are preserved.
type Header struct {
	Name  string
	Value string
}

// Cookie is a cookie set by the response.
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  string
	HTTPOnly bool
	Secure   bool
}

// TraceStage identifies a phase of an HTTP exchange.
type TraceStage int

const (
	TraceConnect TraceStage = iota
	TraceTLS
	TraceSendHeaders
	TraceSendBody
	TraceReceiveHeaders
	TraceReceiveBody
	TraceClosed
)

// TraceStages lists every stage in the order they occur.
var TraceStages = []TraceStage{
	TraceConnect, TraceTLS, TraceSendHeaders, TraceSendBody,
	TraceReceiveHeaders, TraceReceiveBody, TraceClosed,
}

func (s TraceStage) String() string {
	switch s {
	case TraceConnect:
		return "connect"
	case TraceTLS:
		return "tls handshake"
	case TraceSendHeaders:
		return "send headers"
	case TraceSendBody:
		return "send body"
	case TraceReceiveHeaders:
		return "receive headers"
	case TraceReceiveBody:
		return "receive body"
	case TraceClosed:
		return "response closed"
	}
	return "unknown"
}

// TraceState is the progress of one trace stage.
type TraceState int

const (
	TracePending TraceState = iota
	TraceStarted
	TraceComplete
	TraceFailed
	TraceSkipped
)

// TraceEvent records the timing of one stage of the exchange.
type TraceEvent struct {
	Stage    TraceStage
	State    TraceState
	Duration time.Duration
}

// Response is the result of sending a Request.
type Response struct {
	StatusCode int
	Reason     string
	Proto      string
	Headers    []Header
	Cookies    []Cookie
	Body       []byte
	Elapsed    time.Duration
	// ReceivedAt is when the response finished arriving.
	ReceivedAt time.Time
	Trace      []TraceEvent
	// URL is the final, resolved URL (after redirects and variable substitution).
	URL    string
	Method Method
}

// Header returns the first header value matching name case-insensitively.
func (r *Response) Header(name string) string {
	for _, h := range r.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// ContentType returns the media type without parameters (e.g. "application/json").
func (r *Response) ContentType() string {
	ct := r.Header("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

// Size is the body size in bytes.
func (r *Response) Size() int { return len(r.Body) }

// StatusClass buckets a status code for colouring.
type StatusClass int

const (
	StatusClassSuccess StatusClass = iota
	StatusClassRedirect
	StatusClassError
)

// ClassifyStatus maps a status code to success (<300), redirect (<400) or error.
func ClassifyStatus(code int) StatusClass {
	switch {
	case code < 300:
		return StatusClassSuccess
	case code < 400:
		return StatusClassRedirect
	default:
		return StatusClassError
	}
}

// FormatBytes renders a byte count like "1.23 KB".
func FormatBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n) / unit
	for _, suffix := range []string{"KB", "MB", "GB"} {
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
		value /= unit
	}
	return fmt.Sprintf("%.2f TB", value)
}

// FormatDuration renders an elapsed time like "45.67 ms" or "1.20 s".
func FormatDuration(d time.Duration) string {
	ms := float64(d) / float64(time.Millisecond)
	if ms < 1000 {
		return fmt.Sprintf("%.2f ms", ms)
	}
	return fmt.Sprintf("%.2f s", ms/1000)
}
