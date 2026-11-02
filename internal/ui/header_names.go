package ui

import t "github.com/darrenburns/terma"

// commonRequestHeaders are offered as completions for header names.
var commonRequestHeaders = []struct{ name, description string }{
	{"Accept", "Media types the client can handle"},
	{"Accept-Charset", "Character sets the client can handle"},
	{"Accept-Encoding", "Content encodings the client can handle"},
	{"Accept-Language", "Preferred natural languages"},
	{"Authorization", "Credentials for authenticating the client"},
	{"Cache-Control", "Caching directives"},
	{"Connection", "Whether the connection stays open"},
	{"Content-Encoding", "Encoding applied to the body"},
	{"Content-Length", "Size of the body in bytes"},
	{"Content-Type", "Media type of the body"},
	{"Cookie", "Cookies previously sent by the server"},
	{"Date", "When the message was sent"},
	{"Expect", "Expectations the server must meet"},
	{"Forwarded", "Proxy information"},
	{"From", "Email address of the user"},
	{"Host", "Host and port of the server"},
	{"If-Match", "Only act if the ETag matches"},
	{"If-Modified-Since", "Only return if modified since the date"},
	{"If-None-Match", "Only return if the ETag doesn't match"},
	{"If-Range", "Range request precondition"},
	{"If-Unmodified-Since", "Only act if unmodified since the date"},
	{"Max-Forwards", "Limit on proxy forwarding"},
	{"Origin", "Where the request originates"},
	{"Pragma", "Implementation-specific directives"},
	{"Prefer", "Preferred server behaviours"},
	{"Proxy-Authorization", "Credentials for a proxy"},
	{"Range", "Request only part of a resource"},
	{"Referer", "Address of the previous page"},
	{"TE", "Transfer encodings the client accepts"},
	{"Upgrade", "Ask the server to switch protocol"},
	{"User-Agent", "Identifies the client software"},
	{"Via", "Proxies the request passed through"},
	{"X-API-Key", "API key (common convention)"},
	{"X-Correlation-ID", "Correlates requests across services"},
	{"X-Forwarded-For", "Originating client IP address"},
	{"X-Request-ID", "Unique identifier for the request"},
	{"X-Requested-With", "Identifies Ajax requests"},
}

func headerSuggestions() []t.Suggestion {
	out := make([]t.Suggestion, len(commonRequestHeaders))
	for i, h := range commonRequestHeaders {
		out[i] = t.Suggestion{Label: h.name, Value: h.name, Description: h.description}
	}
	return out
}
