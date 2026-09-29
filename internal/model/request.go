// Package model holds Posting's framework-independent data types.
//
// The UI edits these values and hands them to a client.Sender; nothing in this
// package knows about Terma, HTTP transports or the filesystem.
package model

import "strings"

// Method is an HTTP request method.
type Method string

const (
	MethodGet     Method = "GET"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH"
	MethodDelete  Method = "DELETE"
	MethodHead    Method = "HEAD"
	MethodOptions Method = "OPTIONS"
)

// Methods lists every supported method in display order.
var Methods = []Method{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete, MethodHead, MethodOptions}

// Short returns a three letter abbreviation used in compact lists.
func (m Method) Short() string {
	s := string(m)
	if len(s) > 3 {
		return s[:3]
	}
	return s
}

// SortRank orders requests within a collection: GET, POST, PUT, PATCH, DELETE, then the rest.
func (m Method) SortRank() int {
	for i, candidate := range []Method{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete} {
		if m == candidate {
			return i
		}
	}
	return 99
}

// KeyValue is a named value that can be switched off without being deleted.
// It is used for headers, query parameters, path parameters and form fields.
type KeyValue struct {
	Name    string
	Value   string
	Enabled bool
}

// BodyType selects how the request body is encoded.
type BodyType string

const (
	BodyNone BodyType = "none"
	BodyRaw  BodyType = "raw"
	BodyForm BodyType = "form"
)

// Body is the request payload.
type Body struct {
	Type BodyType
	// Raw is used when Type is BodyRaw.
	Raw string
	// ContentType is the MIME type of Raw (e.g. application/json).
	ContentType string
	// Form is used when Type is BodyForm (application/x-www-form-urlencoded).
	Form []KeyValue
}

// AuthType selects the authentication scheme.
type AuthType string

const (
	AuthNone   AuthType = "none"
	AuthBasic  AuthType = "basic"
	AuthDigest AuthType = "digest"
	AuthBearer AuthType = "bearer_token"
)

// Auth holds credentials for every scheme; only the fields for Type are used.
type Auth struct {
	Type     AuthType
	Username string // basic, digest
	Password string // basic, digest
	Token    string // bearer
}

// Options control how a request is sent.
type Options struct {
	FollowRedirects         bool
	VerifySSL               bool
	AttachCookies           bool
	SubstituteBodyVariables bool
	ProxyURL                string
	TimeoutSeconds          float64
}

// DefaultOptions matches Posting 2's defaults.
func DefaultOptions() Options {
	return Options{
		FollowRedirects:         true,
		VerifySSL:               true,
		AttachCookies:           true,
		SubstituteBodyVariables: true,
		TimeoutSeconds:          5,
	}
}

// Scripts are collection-relative paths (optionally "file.py:function") that
// Posting 2 runs around a request. Posting 3 doesn't run them, but keeps them
// so that re-saving a Posting 2 request doesn't lose them.
type Scripts struct {
	Setup      string
	OnRequest  string
	OnResponse string
}

// Request is a complete, unresolved request as the user edited it. Variables
// such as ${BASE_URL} are still present; resolving them is the sender's job.
type Request struct {
	Name        string
	Description string
	Method      Method
	URL         string
	Headers     []KeyValue
	Query       []KeyValue
	PathParams  []KeyValue
	Body        Body
	Auth        Auth
	Options     Options
	Scripts     Scripts
	// File is the collection-relative path of the saved request, or empty
	// when the request has not been saved.
	File string
}

// NewRequest returns an empty GET request with default options.
func NewRequest() Request {
	return Request{
		Method:  MethodGet,
		Body:    Body{Type: BodyNone, ContentType: "application/json"},
		Auth:    Auth{Type: AuthNone},
		Options: DefaultOptions(),
	}
}

// Clone returns a deep copy so edits never alias another request's slices.
func (r Request) Clone() Request {
	r.Headers = cloneKV(r.Headers)
	r.Query = cloneKV(r.Query)
	r.PathParams = cloneKV(r.PathParams)
	r.Body.Form = cloneKV(r.Body.Form)
	return r
}

// DisplayName is the name shown in tabs and lists.
func (r Request) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	if r.URL != "" {
		return r.URL
	}
	return "Untitled"
}

func cloneKV(in []KeyValue) []KeyValue {
	if in == nil {
		return nil
	}
	return append([]KeyValue(nil), in...)
}

// PathParamNames returns the names of ":param" tokens in a URL path, in order
// and without duplicates. "::name" is an escaped literal colon.
func PathParamNames(rawURL string) []string {
	path := rawURL
	if i := strings.Index(path, "://"); i >= 0 {
		path = path[i+3:]
		j := strings.IndexByte(path, '/')
		if j < 0 {
			return nil
		}
		path = path[j:]
	}
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	var names []string
	seen := map[string]bool{}
	for _, segment := range strings.Split(path, "/") {
		if !strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "::") {
			continue
		}
		name := segment[1:]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
