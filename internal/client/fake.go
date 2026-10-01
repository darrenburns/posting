package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/darrenburns/posting/internal/model"
)

// Fake is a Sender that never touches the network. It walks through every
// trace stage with small delays and returns a response echoing the request,
// so the whole UI can be exercised before a real client exists.
type Fake struct {
	// StageDelay is how long each trace stage takes. Zero means no delay.
	StageDelay time.Duration
}

// Send simulates a request.
func (f Fake) Send(ctx context.Context, call Call) (*model.Response, error) {
	started := time.Now()
	req, ok := model.Lower(call.Request)
	if !ok {
		return nil, fmt.Errorf("%s requests aren't sent over HTTP", call.Request.Kind().Label)
	}
	resolvedURL := model.Substitute(model.ResolvePathParams(req.URL, req.PathParams), call.Lookup)
	parsed, err := url.Parse(resolvedURL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", resolvedURL)
	}

	var trace []model.TraceEvent
	for _, stage := range model.TraceStages {
		if stage == model.TraceTLS && parsed.Scheme != "https" {
			trace = append(trace, model.TraceEvent{Stage: stage, State: model.TraceSkipped})
			f.report(call, trace[len(trace)-1])
			continue
		}
		f.report(call, model.TraceEvent{Stage: stage, State: model.TraceStarted})
		stageStart := time.Now()
		select {
		case <-ctx.Done():
			f.report(call, model.TraceEvent{Stage: stage, State: model.TraceFailed})
			return nil, ctx.Err()
		case <-time.After(f.StageDelay):
		}
		event := model.TraceEvent{Stage: stage, State: model.TraceComplete, Duration: time.Since(stageStart)}
		trace = append(trace, event)
		f.report(call, event)
	}

	headers := map[string]string{}
	for _, h := range req.Headers {
		if h.Enabled {
			headers[h.Name] = model.Substitute(h.Value, call.Lookup)
		}
	}
	query := map[string]string{}
	for key, values := range parsed.Query() {
		query[key] = strings.Join(values, ",")
	}
	echo := map[string]any{
		"method":  string(req.Method),
		"url":     resolvedURL,
		"args":    query,
		"headers": headers,
	}
	switch req.Body.Type {
	case model.BodyRaw:
		var decoded any
		if json.Unmarshal([]byte(req.Body.Raw), &decoded) == nil {
			echo["json"] = decoded
		} else {
			echo["data"] = req.Body.Raw
		}
	case model.BodyForm:
		form := map[string]string{}
		for _, item := range req.Body.Form {
			if item.Enabled {
				form[item.Name] = item.Value
			}
		}
		echo["form"] = form
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(echo)
	body := bytes.TrimSpace(encoded.Bytes())

	status, reason := 200, "OK"
	switch req.Method {
	case model.MethodPost:
		status, reason = 201, "Created"
	case model.MethodDelete:
		status, reason = 204, "No Content"
	}
	if strings.Contains(parsed.Path, "404") || strings.Contains(parsed.Path, "missing") {
		status, reason = 404, "Not Found"
	}
	if req.Method == model.MethodHead || status == 204 {
		body = nil
	}

	return &model.Response{
		StatusCode: status,
		Reason:     reason,
		Proto:      "HTTP/1.1",
		Headers: []model.Header{
			{Name: "Content-Type", Value: "application/json; charset=utf-8"},
			{Name: "Content-Length", Value: fmt.Sprint(len(body))},
			{Name: "Date", Value: time.Now().UTC().Format(time.RFC1123)},
			{Name: "Server", Value: "posting-fake/3.0"},
			{Name: "Set-Cookie", Value: "session=fake-session-id; Path=/; HttpOnly"},
		},
		Cookies: []model.Cookie{
			{Name: "session", Value: "fake-session-id", Path: "/", HTTPOnly: true},
		},
		Body:       body,
		Elapsed:    time.Since(started),
		ReceivedAt: time.Now(),
		Trace:      trace,
		URL:        resolvedURL,
		Method:     req.Method,
	}, nil
}

func (f Fake) report(call Call, event model.TraceEvent) {
	if call.OnTrace != nil {
		call.OnTrace(event)
	}
}
