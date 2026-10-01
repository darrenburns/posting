package model

import (
	"encoding/json"
	"strconv"
)

// KindID names a kind of request. It is the value of a request file's
// `kind:` key; a file without one is HTTP.
type KindID string

const (
	KindHTTP    KindID = "http"
	KindGraphQL KindID = "graphql"
)

// Fields is a set of the shared Request fields a kind uses. Normalize resets
// the fields outside a kind's set, so they are never saved or sent for it,
// and the request panel only shows tabs for the fields in it.
type Fields uint32

const (
	FieldMethod Fields = 1 << iota
	FieldBody
	FieldQuery
	FieldPathParams
	FieldHeaders
	FieldAuth

	AllFields = FieldAuth<<1 - 1
)

// Has reports whether f includes every field in field.
func (f Fields) Has(field Fields) bool { return f&field == field }

// Kind is what Posting knows about one kind of request that isn't tied to a
// UI toolkit or a file format. Behaviour that varies per request lives on
// the kind's Payload type instead, where the compiler checks it is all there.
type Kind struct {
	ID    KindID
	Label string // "HTTP", "GraphQL"
	// Badge is the three letter label lists show. HTTP shows the method instead.
	Badge  string
	Fields Fields

	// zero is an empty payload of this kind; nil for HTTP.
	zero Payload
	// decode reads a payload from history JSON.
	decode  func(data []byte) (Payload, error)
	example func() Request
	rank    int
}

var (
	HTTPKind = &Kind{
		ID: KindHTTP, Label: "HTTP", Fields: AllFields,
		example: exampleHTTP,
	}
	GraphQLKind = &Kind{
		ID: KindGraphQL, Label: "GraphQL", Badge: "GQL",
		// Lowering sets the method and body, so they aren't the user's to edit.
		Fields:  AllFields &^ (FieldMethod | FieldBody),
		zero:    GraphQL{},
		decode:  decodePayload[GraphQL],
		example: exampleGraphQL,
		rank:    1,
	}
)

// Kinds lists every kind in menu order. Each layer that keeps a table keyed
// by kind has a test ranging over this list, so a new kind added here turns
// those tests red until every layer knows about it.
var Kinds = []*Kind{HTTPKind, GraphQLKind}

// KindByID finds a kind by its file name.
func KindByID(id KindID) (*Kind, bool) {
	for _, k := range Kinds {
		if k.ID == id {
			return k, true
		}
	}
	return nil, false
}

// New returns an empty request of kind k with default options.
func (k *Kind) New() Request {
	r := NewRequest()
	r.Payload = k.zero
	return Normalize(r)
}

// Example returns a filled-in request of kind k, for tests that must
// exercise every field a kind's codecs handle.
func (k *Kind) Example() Request { return k.example() }

// OverHTTP reports whether requests of this kind are sent as HTTP requests
// (see Lower).
func (k *Kind) OverHTTP() bool {
	if k.zero == nil {
		return true
	}
	_, ok := k.zero.(HTTPCarried)
	return ok
}

// Payload is a kind's own request data, such as a GraphQL document. The
// interface is sealed, so every payload lives in this package and a payload
// type missing a method does not compile.
type Payload interface {
	Kind() *Kind
	clone() Payload
	// resolve substitutes variables by the kind's own rules. braced replaces
	// only ${NAME} references; all replaces $NAME too, as raw bodies do.
	resolve(braced, all func(string) string, opts Options) Payload
	// status reads an exchange's outcome the way the kind's users think of
	// it: a GraphQL 200 with errors is not a plain success.
	status(resp *Response) Status
	// size is the payload's size in bytes, for the history budget.
	size() int
}

// HTTPCarried is a payload sent as an HTTP request. Lower builds that
// request from the envelope (URL, headers, auth, options) and the payload.
type HTTPCarried interface {
	Payload
	Lower(envelope Request) Request
}

// Normalize resets the shared fields r's kind doesn't use to NewRequest's
// defaults, so a GraphQL request never carries a method or an HTTP body into
// a file, the sender or history. It is the identity for HTTP requests, which
// keeps Posting 2 files byte-identical, and it is idempotent.
func Normalize(r Request) Request {
	fields := r.Kind().Fields
	if fields == AllFields {
		return r
	}
	defaults := NewRequest()
	if !fields.Has(FieldMethod) {
		r.Method = defaults.Method
	}
	if !fields.Has(FieldBody) {
		r.Body = defaults.Body
	}
	if !fields.Has(FieldQuery) {
		r.Query = nil
	}
	if !fields.Has(FieldPathParams) {
		r.PathParams = nil
	}
	if !fields.Has(FieldHeaders) {
		r.Headers = nil
	}
	if !fields.Has(FieldAuth) {
		r.Auth = defaults.Auth
	}
	return r
}

// Lower returns the HTTP request that carries r: r itself for HTTP, the
// payload's lowering for other HTTP-carried kinds. ok is false for a kind
// with its own transport. Lower after Resolve: once a GraphQL request's
// variables are encoded into its JSON body, the query's ${NAME}s can't be
// told apart from theirs.
func Lower(r Request) (wire Request, ok bool) {
	if r.Payload == nil {
		return r, true
	}
	carried, ok := r.Payload.(HTTPCarried)
	if !ok {
		return Request{}, false
	}
	return carried.Lower(r), true
}

// Status is the headline outcome of an exchange.
type Status struct {
	Code  string // "200"
	Text  string // "OK", "2 errors"
	Class StatusClass
}

// StatusOf reads resp the way req's kind understands it. Displays of an
// exchange's outcome read this rather than resp.StatusCode.
func StatusOf(req Request, resp *Response) Status {
	if req.Payload == nil {
		return httpStatus(resp)
	}
	return req.Payload.status(resp)
}

func httpStatus(resp *Response) Status {
	return Status{Code: strconv.Itoa(resp.StatusCode), Text: resp.Reason, Class: ClassifyStatus(resp.StatusCode)}
}

func decodePayload[T Payload](data []byte) (Payload, error) {
	var p T
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func exampleHTTP() Request {
	r := NewRequest()
	r.Name = "Create user"
	r.Description = "Create a new user account."
	r.Method = MethodPost
	r.URL = "${BASE_URL}/users/:team"
	r.Headers = []KeyValue{{Name: "Content-Type", Value: "application/json", Enabled: true}}
	r.Query = []KeyValue{{Name: "notify", Value: "true", Enabled: true}}
	r.PathParams = []KeyValue{{Name: "team", Value: "core", Enabled: true}}
	r.Body = Body{Type: BodyRaw, Raw: "{\n  \"name\": \"Ada\"\n}", ContentType: "application/json"}
	r.Auth = Auth{Type: AuthBearer, Token: "${API_TOKEN}"}
	r.Options.TimeoutSeconds = 30
	r.File = "users/create-user.posting.yaml"
	return r
}
