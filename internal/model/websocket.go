package model

import "strconv"

// WebSocket is a draft message. Connecting in the UI does not send it;
// subsequent sends use its current contents. Binary messages are base64.
type WebSocket struct {
	Message string
	Binary  bool
}

func (w WebSocket) Kind() *Kind    { return WebSocketKind }
func (w WebSocket) clone() Payload { return w }
func (w WebSocket) size() int      { return len(w.Message) }
func (w WebSocket) label() string  { return "" }
func (w WebSocket) resolve(_, all func(string) string, opts Options) Payload {
	if opts.SubstituteBodyVariables {
		w.Message = all(w.Message)
	}
	return w
}

func (w WebSocket) status(resp *Response) Status {
	if resp.WebSocket == nil {
		return httpStatus(resp)
	}
	close := resp.WebSocket
	class := StatusClassSuccess
	if close.Code != 1000 && close.Code != 1001 {
		class = StatusClassError
	}
	text := "Disconnected"
	if close.Reason != "" {
		text = close.Reason
	}
	return Status{Code: strconv.Itoa(close.Code), Text: text, Class: class}
}

// WebSocketStatus records the close code and reason, not the HTTP upgrade.
type WebSocketStatus struct {
	Code   int
	Reason string
}

func exampleWebSocket() Request {
	r := NewRequest()
	r.Name, r.URL = "Deployment updates", "wss://api.example.com/events/:team"
	r.Payload = WebSocket{Message: `{"subscribe":"${USER_ID}"}`}
	r.Headers = []KeyValue{{Name: "Sec-WebSocket-Protocol", Value: "events.v1", Enabled: true}}
	r.PathParams = []KeyValue{{Name: "team", Value: "core", Enabled: true}}
	r.Query = []KeyValue{{Name: "region", Value: "eu", Enabled: false}}
	r.Auth = Auth{Type: AuthBearer, Token: "${API_TOKEN}"}
	r.File = "events.posting.yaml"
	return r
}
