package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQL is a GraphQL operation, sent over HTTP as a POST with a JSON body.
type GraphQL struct {
	// Query is the GraphQL document. Its $name references are GraphQL
	// variables, so only ${NAME} Posting variables are substituted in it.
	Query string
	// Variables is a JSON object as the user typed it. It is text rather than
	// a map so it can hold references that only become valid JSON once
	// substituted, and keeps the user's formatting.
	Variables string
	// OperationName picks one operation when Query defines several.
	OperationName string
}

var _ HTTPCarried = GraphQL{}

func (g GraphQL) Kind() *Kind { return GraphQLKind }

func (g GraphQL) clone() Payload { return g }

func (g GraphQL) size() int { return len(g.Query) + len(g.Variables) + len(g.OperationName) }

func (g GraphQL) label() string { return g.OperationName }

func (g GraphQL) resolve(braced, all func(string) string, opts Options) Payload {
	if opts.SubstituteBodyVariables {
		g.Query = braced(g.Query)
		g.Variables = all(g.Variables)
	}
	return g
}

// graphQLAccept is sent when the request has no Accept header of its own:
// the GraphQL over HTTP media type, with plain JSON for older servers.
const graphQLAccept = "application/graphql-response+json, application/json"

// Lower is a POST of {"query", "variables", "operationName"} as JSON, with
// the envelope's URL, headers, auth and options.
func (g GraphQL) Lower(envelope Request) Request {
	wire := envelope.Clone()
	wire.Payload = nil
	wire.Method = MethodPost
	wire.Body = Body{Type: BodyRaw, ContentType: "application/json", Raw: g.document()}
	if !hasEnabledHeader(wire.Headers, "Accept") {
		wire.Headers = append(wire.Headers, KeyValue{Name: "Accept", Value: graphQLAccept, Enabled: true})
	}
	return wire
}

// document is the JSON request body. Blank variables and operation name are
// left out. Variables that aren't valid JSON are spliced in as written, so
// the server reports them, as it would for a raw JSON body.
func (g GraphQL) document() string {
	var b strings.Builder
	b.WriteString(`{"query":`)
	b.WriteString(jsonString(g.Query))
	if vars := strings.TrimSpace(g.Variables); vars != "" {
		b.WriteString(`,"variables":`)
		var compact bytes.Buffer
		if json.Compact(&compact, []byte(vars)) == nil {
			b.Write(compact.Bytes())
		} else {
			b.WriteString(vars)
		}
	}
	if g.OperationName != "" {
		b.WriteString(`,"operationName":`)
		b.WriteString(jsonString(g.OperationName))
	}
	b.WriteString("}")
	return b.String()
}

// status is the HTTP status, made a warning when a 2xx body has a
// non-empty top-level "errors" array.
func (g GraphQL) status(resp *Response) Status {
	s := httpStatus(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s
	}
	var body struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(resp.Body, &body) != nil || len(body.Errors) == 0 {
		return s
	}
	s.Class = StatusClassWarning
	s.Text = "1 error"
	if n := len(body.Errors); n > 1 {
		s.Text = fmt.Sprintf("%d errors", n)
	}
	return s
}

// jsonString encodes s as a JSON string without escaping <, > and &, so
// copied curl commands stay readable.
func jsonString(s string) string {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

func hasEnabledHeader(headers []KeyValue, name string) bool {
	for _, h := range headers {
		if h.Enabled && strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

func exampleGraphQL() Request {
	r := NewRequest()
	r.Payload = GraphQL{
		Query:         "query User($id: ID!) {\n  user(id: $id) {\n    name\n    email\n  }\n}\n",
		Variables:     "{\"id\": \"${USER_ID}\"}\n",
		OperationName: "User",
	}
	r.Name = "Get user"
	r.Description = "Fetch a user through the GraphQL API."
	r.URL = "${BASE_URL}/graphql"
	r.Headers = []KeyValue{{Name: "X-Request-Source", Value: "posting", Enabled: true}}
	r.Auth = Auth{Type: AuthBearer, Token: "${API_TOKEN}"}
	r.Options.FollowRedirects = false
	r.File = "users/get-user-graphql.posting.yaml"
	return Normalize(r)
}
