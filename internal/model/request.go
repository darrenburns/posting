// Package model holds Posting's framework-independent data types.
//
// The UI edits these values and hands them to a client.Sender; nothing in this
// package knows about Terma, HTTP transports or the filesystem.
package model

import (
	"maps"
	"slices"
	"strings"
)

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
	BodyNone      BodyType = "none"
	BodyRaw       BodyType = "raw"
	BodyForm      BodyType = "form"
	BodyMultipart BodyType = "multipart"
	BodyFile      BodyType = "file"
)

// PartType distinguishes inline text from a file reference, including an
// unfinished file part whose path is still empty.
type PartType string

const (
	PartText PartType = "text"
	PartFile PartType = "file"
)

// Part is one ordered multipart/form-data part. Duplicate names are kept.
// Filename and ContentType are optional overrides; file contents are never
// stored in the request or substituted as variables.
type Part struct {
	Type        PartType
	Name        string
	Value       string
	File        string
	Filename    string
	ContentType string
	Enabled     bool
}

// Body is the request payload.
type Body struct {
	Type BodyType
	// Raw is used when Type is BodyRaw.
	Raw string
	// ContentType is the MIME type of Raw or File. Empty means infer it.
	ContentType string
	// Form is used when Type is BodyForm (application/x-www-form-urlencoded).
	Form []KeyValue
	// Multipart is used when Type is BodyMultipart.
	Multipart []Part `json:",omitempty"`
	// File is a collection-relative or absolute path for BodyFile.
	File string `json:",omitempty"`
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
	// Captures maps session variable names to paths in the JSON response body.
	Captures map[string]string `json:",omitempty"`
	// Payload is the part of the request only its kind understands, such as
	// a GraphQL document. Nil is a plain HTTP request. The kind is derived
	// from Payload's type rather than stored beside it, so they can't disagree.
	Payload Payload
	// File is the collection-relative path of the saved request, or empty
	// when the request has not been saved.
	File string

	VariableScope *VariableScope `json:",omitempty"`
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
	r.VariableScope = r.VariableScope.Clone()
	r.Captures = maps.Clone(r.Captures)
	r.Headers = cloneKV(r.Headers)
	r.Query = cloneKV(r.Query)
	r.PathParams = cloneKV(r.PathParams)
	r.Body.Form = cloneKV(r.Body.Form)
	r.Body.Multipart = slices.Clone(r.Body.Multipart)
	if r.Payload != nil {
		r.Payload = r.Payload.clone()
	}
	return r
}

// Kind is the request's kind: HTTPKind when it has no payload.
func (r Request) Kind() *Kind {
	if r.Payload == nil {
		return HTTPKind
	}
	return r.Payload.Kind()
}

// Badge is the short label lists show beside a request: the method for
// HTTP ("GET", "POS"), the kind's badge otherwise ("GQL").
func (r Request) Badge() string {
	if r.Payload == nil {
		return r.Method.Short()
	}
	return r.Kind().Badge
}

// SortRank orders requests in a collection: HTTP by method, then other
// kinds in registry order.
func (r Request) SortRank() int {
	if r.Payload == nil {
		return r.Method.SortRank()
	}
	return 100 + slices.Index(Kinds, r.Kind())
}

// PayloadSize is the size in bytes of what the request sends beyond its
// URL and headers: the raw body for HTTP, the payload otherwise.
func (r Request) PayloadSize() int {
	if r.Payload == nil {
		n := len(r.Body.Raw) + len(r.Body.File)
		for _, field := range r.Body.Form {
			n += len(field.Name) + len(field.Value)
		}
		for _, p := range r.Body.Multipart {
			n += len(p.Name) + len(p.Value) + len(p.File) + len(p.Filename) + len(p.ContentType)
		}
		return n
	}
	return r.Payload.size()
}

// DisplayName is the name shown in tabs and lists: the request's name, its
// payload's label, its URL, or "Untitled".
func (r Request) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	if r.Payload != nil {
		if label := r.Payload.label(); label != "" {
			return label
		}
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
