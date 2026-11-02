package client

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

// Bound unfinished events as well as the retained transcript. A peer that
// never sends a blank line must not grow the parser without limit.
const maxSSEEventBytes = 1 << 20

// receiveSSE retains the decoded wire body for export and completed events
// for inspection. Stopping keeps both; reconnecting is always a new send, so
// a POST is never replayed automatically.
func receiveSSE(ctx context.Context, call Call, resp *http.Response, trace *tracer, started time.Time, timeout float64) *model.Response {
	out := model.Response{
		StatusCode: resp.StatusCode, Reason: reason(resp), Proto: resp.Proto,
		Headers: headers(resp.Header), Cookies: cookies(resp.Cookies()),
		URL: resp.Request.URL.String(), Method: model.Method(resp.Request.Method),
		Events: []model.MessageEvent{{At: time.Now(), Kind: model.MessageInfo, Title: "Stream connected"}},
	}
	report := func() {
		if call.OnUpdate != nil {
			snapshot := out
			snapshot.Events = slices.Clip(out.Events)
			snapshot.Elapsed = time.Since(started)
			snapshot.Trace = trace.events()
			call.OnUpdate(&snapshot)
		}
	}
	report() // Show headers and Stop even before the first event arrives.

	var raw bytes.Buffer
	events := make(chan model.MessageEvent)
	done := make(chan error, 1)
	go func() {
		reader, err := sseBody(resp)
		if err == nil {
			defer reader.Close()
			limited := &io.LimitedReader{R: reader, N: maxBodyBytes + 1}
			err = parseSSE(io.TeeReader(limited, &raw), func(event model.MessageEvent) { events <- event })
			if limited.N == 0 {
				err = fmt.Errorf("SSE response exceeded %s", model.FormatBytes(maxBodyBytes))
			}
		}
		done <- err
		close(events)
	}()

	// Parsing can continue while the UI redraws at most ten times a second.
	// Only this goroutine owns out; published event prefixes are immutable.
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	dirty := false
	for events != nil {
		select {
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			out.Events = append(out.Events, event)
			dirty = true
		case <-ticker.C:
			if dirty {
				report()
				dirty = false
			}
		}
	}
	err := <-done
	out.Body = raw.Bytes()[:min(raw.Len(), maxBodyBytes)]
	title, kind := "Stream ended", model.MessageInfo
	switch {
	case errors.Is(context.Cause(ctx), context.Canceled):
		title = "Stream stopped"
		if call.OnUpdate == nil {
			out.StreamError = context.Canceled.Error()
		}
	case err != nil:
		out.StreamError = describeError(ctx, err, timeout).Error()
		title, kind = "Stream interrupted: "+out.StreamError, model.MessageError
	}
	out.Events = append(out.Events, model.MessageEvent{At: time.Now(), Kind: kind, Title: title})
	if out.StreamError != "" {
		trace.fail()
	} else {
		trace.complete(model.TraceReceiveBody)
	}
	trace.start(model.TraceClosed)
	resp.Body.Close()
	trace.complete(model.TraceClosed)
	out.Elapsed, out.ReceivedAt, out.Trace = time.Since(started), time.Now(), trace.events()
	return &out
}

// sseBody decodes incrementally: buffering compressed data until EOF would
// hide events on a long-lived connection. Deflate accepts both HTTP's zlib
// wrapper and the raw streams accepted by ordinary HTTP responses.
func sseBody(resp *http.Response) (io.ReadCloser, error) {
	switch encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))); encoding {
	case "", "identity":
		return resp.Body, nil
	case "gzip", "x-gzip":
		return gzip.NewReader(resp.Body)
	case "deflate":
		reader := bufio.NewReader(resp.Body)
		header, err := reader.Peek(2)
		if err != nil {
			return nil, err
		}
		if header[0]&0x0f == 8 && header[0]>>4 <= 7 && (int(header[0])<<8|int(header[1]))%31 == 0 {
			return zlib.NewReader(reader)
		}
		return flate.NewReader(reader), nil
	default:
		return nil, fmt.Errorf("unsupported SSE content encoding %q", encoding)
	}
}

// parseSSE implements the event-stream framing rules. Comments and unknown
// fields are ignored, IDs persist, and EOF never dispatches a partial event.
// retry is intentionally ignored because Posting doesn't auto-reconnect.
func parseSSE(reader io.Reader, emit func(model.MessageEvent)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxSSEEventBytes+1)
	// Consume CR immediately, then swallow an optional LF on the next scan.
	// Waiting for the byte after CR can delay a complete event indefinitely.
	skipLF := false
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		start := 0
		if skipLF && len(data) > 0 {
			skipLF = false
			if data[0] == '\n' {
				start = 1
			}
		}
		if i := bytes.IndexAny(data[start:], "\r\n"); i >= 0 {
			i += start
			skipLF = data[i] == '\r'
			return i + 1, data[start:i], nil
		}
		if atEOF {
			return len(data), nil, nil
		}
		return start, nil, nil
	})
	var data strings.Builder
	var name, id string
	first, count, size := true, 0, 0
	for scanner.Scan() {
		line := strings.ToValidUTF8(scanner.Text(), "\uFFFD")
		if first {
			line = strings.TrimPrefix(line, "\uFEFF")
			first = false
		}
		if line == "" {
			if data.Len() > 0 {
				if count == maxLogEvents {
					return fmt.Errorf("SSE log reached %d events", maxLogEvents)
				}
				body := []byte(strings.TrimSuffix(data.String(), "\n"))
				contentType := "text/plain"
				if json.Valid(body) {
					contentType = "application/json"
				}
				if name == "" {
					name = "message"
				}
				emit(model.MessageEvent{At: time.Now(), Kind: model.MessageReceived, Body: body, ContentType: contentType, Event: name, ID: id})
				count++
			}
			data.Reset()
			name, size = "", 0
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		size += len(line)
		if size > maxSSEEventBytes {
			return fmt.Errorf("SSE event exceeded %s", model.FormatBytes(maxSSEEventBytes))
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
		case "event":
			name = value
		case "id":
			if !strings.ContainsRune(value, '\x00') {
				id = value
			}
		}
	}
	if errors.Is(scanner.Err(), bufio.ErrTooLong) {
		return fmt.Errorf("SSE line exceeded %s", model.FormatBytes(maxSSEEventBytes))
	}
	return scanner.Err()
}
