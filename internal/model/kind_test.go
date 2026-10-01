package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestKindsAreComplete(t *testing.T) {
	ids, labels := map[KindID]bool{}, map[string]bool{}
	for _, k := range Kinds {
		t.Run(string(k.ID), func(t *testing.T) {
			if k.ID == "" || k.Label == "" {
				t.Fatalf("kind %+v needs an ID and a label", k)
			}
			if ids[k.ID] || labels[k.Label] {
				t.Fatalf("kind %q or label %q is not unique", k.ID, k.Label)
			}
			ids[k.ID], labels[k.Label] = true, true
			if found, ok := KindByID(k.ID); !ok || found != k {
				t.Fatalf("KindByID(%q) = %v, %v", k.ID, found, ok)
			}
			if k != HTTPKind && len(k.Badge) != 3 {
				t.Errorf("badge %q is not three letters", k.Badge)
			}
			if got := k.New().Kind(); got != k {
				t.Errorf("New().Kind() = %s", got.ID)
			}
			example := k.Example()
			if got := example.Kind(); got != k {
				t.Fatalf("Example().Kind() = %s", got.ID)
			}
			if k != HTTPKind && reflect.DeepEqual(example.Payload, k.New().Payload) {
				t.Errorf("example payload is empty, so codec tests wouldn't exercise it")
			}
			once := Normalize(example)
			if twice := Normalize(once); !reflect.DeepEqual(once, twice) {
				t.Errorf("Normalize is not idempotent:\n%+v\n%+v", once, twice)
			}
			if !reflect.DeepEqual(once, example) {
				t.Errorf("Example() is not normalized")
			}
			if k.OverHTTP() {
				wire, ok := Lower(example)
				if !ok || wire.Payload != nil || wire.Method == "" {
					t.Errorf("Lower(example) = %+v, %v; want a plain HTTP request", wire, ok)
				}
			}
			resolved, err := Resolve(example, MapLookup(map[string]string{"BASE_URL": "http://x", "API_TOKEN": "t", "USER_ID": "7"}))
			if err != nil || resolved.Kind() != k {
				t.Errorf("Resolve kept kind %v, err %v", resolved.Kind(), err)
			}
			if k.decode == nil && k.zero != nil {
				t.Errorf("kind has a payload but no JSON decoder")
			}
		})
	}
}

func TestNormalizeIsIdentityForHTTP(t *testing.T) {
	r := HTTPKind.Example()
	r.Method = MethodOptions
	r.Body.Form = []KeyValue{{Name: "a", Value: "b"}}
	if got := Normalize(r); !reflect.DeepEqual(got, r) {
		t.Fatalf("Normalize changed an HTTP request:\n%+v\n%+v", got, r)
	}
}

func TestNormalizeClearsFieldsGraphQLDoesNotUse(t *testing.T) {
	r := GraphQLKind.Example()
	r.Method = MethodDelete
	r.Body = Body{Type: BodyRaw, Raw: "stray", ContentType: "text/plain"}
	got := Normalize(r)
	if got.Method != MethodGet || !reflect.DeepEqual(got.Body, Body{Type: BodyNone, ContentType: "application/json"}) {
		t.Fatalf("Normalize kept method %q and body %+v", got.Method, got.Body)
	}
	if !reflect.DeepEqual(got.Headers, r.Headers) || got.Auth != r.Auth || got.URL != r.URL {
		t.Fatalf("Normalize changed fields GraphQL uses: %+v", got)
	}
}

func TestLowerGraphQL(t *testing.T) {
	r := GraphQLKind.New()
	r.URL = "http://api.test/graphql"
	r.Payload = GraphQL{Query: "{ whoami }"}
	wire, ok := Lower(r)
	if !ok {
		t.Fatal("GraphQL requests are carried over HTTP")
	}
	if wire.Payload != nil || wire.Method != MethodPost || wire.URL != r.URL {
		t.Fatalf("wire request = %+v", wire)
	}
	if !reflect.DeepEqual(wire.Body, Body{Type: BodyRaw, ContentType: "application/json", Raw: `{"query":"{ whoami }"}`}) {
		t.Fatalf("blank variables and operation name must be left out, body = %+v", wire.Body)
	}
	want := []KeyValue{{Name: "Accept", Value: "application/graphql-response+json, application/json", Enabled: true}}
	if !reflect.DeepEqual(wire.Headers, want) {
		t.Fatalf("headers = %+v, want %+v", wire.Headers, want)
	}
	if r.Headers != nil {
		t.Fatalf("Lower changed the envelope's headers: %+v", r.Headers)
	}
}

func TestLowerGraphQLBody(t *testing.T) {
	cases := []struct {
		name string
		g    GraphQL
		want string
	}{
		{"valid variables are compacted", GraphQL{Query: "query Q($id: ID!) { user(id: $id) { name } }", Variables: "{\n  \"id\": \"7\"\n}\n", OperationName: "Q"},
			`{"query":"query Q($id: ID!) { user(id: $id) { name } }","variables":{"id":"7"},"operationName":"Q"}`},
		{"invalid variables are spliced as written", GraphQL{Query: "{ a }", Variables: "{id: 7}"},
			`{"query":"{ a }","variables":{id: 7}}`},
		{"whitespace variables are blank", GraphQL{Query: "{ a }", Variables: " \n"},
			`{"query":"{ a }"}`},
		{"query text is JSON escaped without HTML escaping", GraphQL{Query: "{ a(s: \"<&>\") }\n"},
			`{"query":"{ a(s: \"<&>\") }\n"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := GraphQLKind.New()
			r.Payload = c.g
			wire, _ := Lower(r)
			if wire.Body.Raw != c.want {
				t.Fatalf("body = %s\nwant   %s", wire.Body.Raw, c.want)
			}
		})
	}
}

func TestLowerKeepsAnEnabledAcceptHeader(t *testing.T) {
	r := GraphQLKind.New()
	r.Payload = GraphQL{Query: "{ a }"}
	r.Headers = []KeyValue{{Name: "accept", Value: "application/json", Enabled: true}}
	wire, _ := Lower(r)
	if !reflect.DeepEqual(wire.Headers, r.Headers) {
		t.Fatalf("an enabled Accept header must not be joined by another: %+v", wire.Headers)
	}

	r.Headers[0].Enabled = false
	wire, _ = Lower(r)
	if len(wire.Headers) != 2 || wire.Headers[1].Value != graphQLAccept {
		t.Fatalf("a disabled Accept header doesn't count: %+v", wire.Headers)
	}
}

func TestLowerIsIdentityForHTTP(t *testing.T) {
	r := HTTPKind.Example()
	wire, ok := Lower(r)
	if !ok || !reflect.DeepEqual(wire, r) {
		t.Fatalf("Lower changed an HTTP request: %+v", wire)
	}
}

func TestResolveGraphQLKeepsGraphQLVariables(t *testing.T) {
	r := GraphQLKind.New()
	r.URL = "${BASE_URL}/graphql"
	r.Payload = GraphQL{
		Query:         "query User($id: ID!) { user(id: $id, tenant: \"${TENANT}\") { name } }",
		Variables:     `{"id": "$id", "tenant": "${TENANT}"}`,
		OperationName: "${TENANT}",
	}
	env := MapLookup(map[string]string{"BASE_URL": "http://api.test", "id": "LEAKED", "TENANT": "acme"})
	resolved, err := Resolve(r, env)
	if err != nil {
		t.Fatal(err)
	}
	got := resolved.Payload.(GraphQL)
	want := GraphQL{
		Query:         "query User($id: ID!) { user(id: $id, tenant: \"acme\") { name } }",
		Variables:     `{"id": "LEAKED", "tenant": "acme"}`,
		OperationName: "${TENANT}",
	}
	if got != want {
		t.Fatalf("resolved payload = %+v\nwant %+v", got, want)
	}
	if r.Payload.(GraphQL).Query != "query User($id: ID!) { user(id: $id, tenant: \"${TENANT}\") { name } }" {
		t.Fatalf("Resolve changed the original payload")
	}

	r.Options.SubstituteBodyVariables = false
	resolved, _ = Resolve(r, env)
	if resolved.Payload != r.Payload {
		t.Fatalf("with body substitution off the payload is sent as written, got %+v", resolved.Payload)
	}
}

func TestSubstituteBraced(t *testing.T) {
	got := SubstituteBraced("$id ${ID} $$ ${MISSING}", lookup(map[string]string{"id": "x", "ID": "7"}))
	if want := "$id 7 $ ${MISSING}"; got != want {
		t.Fatalf("SubstituteBraced = %q, want %q", got, want)
	}
}

func TestStatusOf(t *testing.T) {
	gql := GraphQLKind.New()
	cases := []struct {
		name string
		req  Request
		resp Response
		want Status
	}{
		{"http", NewRequest(), Response{StatusCode: 200, Reason: "OK", Body: []byte(`{"errors":[{}]}`)}, Status{"200", "OK", StatusClassSuccess}},
		{"graphql ok", gql, Response{StatusCode: 200, Reason: "OK", Body: []byte(`{"data":{}}`)}, Status{"200", "OK", StatusClassSuccess}},
		{"graphql one error", gql, Response{StatusCode: 200, Reason: "OK", Body: []byte(`{"errors":[{"message":"no"}],"data":null}`)}, Status{"200", "1 error", StatusClassWarning}},
		{"graphql errors", gql, Response{StatusCode: 200, Reason: "OK", Body: []byte(`{"errors":[{},{}]}`)}, Status{"200", "2 errors", StatusClassWarning}},
		{"graphql empty errors", gql, Response{StatusCode: 200, Reason: "OK", Body: []byte(`{"errors":[]}`)}, Status{"200", "OK", StatusClassSuccess}},
		{"graphql not json", gql, Response{StatusCode: 200, Reason: "OK", Body: []byte(`<html>`)}, Status{"200", "OK", StatusClassSuccess}},
		{"graphql 400 keeps the http status", gql, Response{StatusCode: 400, Reason: "Bad Request", Body: []byte(`{"errors":[{}]}`)}, Status{"400", "Bad Request", StatusClassError}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatusOf(c.req, &c.resp); got != c.want {
				t.Fatalf("StatusOf = %+v, want %+v", got, c.want)
			}
		})
	}
	if got := StatusOf(gql, nil); got != (Status{}) {
		t.Fatalf("StatusOf with no response = %+v", got)
	}
}

func TestRequestJSONRoundTrip(t *testing.T) {
	for _, k := range Kinds {
		t.Run(string(k.ID), func(t *testing.T) {
			want := k.Example()
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got Request
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip:\n got %+v\nwant %+v", got, want)
			}
			if hasKind := strings.Contains(string(data), `"Kind"`); hasKind != (k != HTTPKind) {
				t.Fatalf("history JSON %s: Kind key present = %v", data, hasKind)
			}
		})
	}
}

// legacyRequest is Request as it was before kinds, so the test below can
// check HTTP history entries still encode the same way.
type legacyRequest struct {
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
	File        string
}

func TestHTTPRequestJSONIsUnchanged(t *testing.T) {
	r := HTTPKind.Example()
	r.Scripts = Scripts{Setup: "setup.py"}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(legacyRequest{r.Name, r.Description, r.Method, r.URL, r.Headers, r.Query, r.PathParams, r.Body, r.Auth, r.Options, r.Scripts, r.File})
	if string(got) != string(want) {
		t.Fatalf("HTTP history JSON changed:\n got %s\nwant %s", got, want)
	}

	var decoded Request
	if err := json.Unmarshal(want, &decoded); err != nil || !reflect.DeepEqual(decoded, r) {
		t.Fatalf("pre-kinds history entry decoded as %+v, %v", decoded, err)
	}
}

func TestRequestJSONRejectsUnknownKind(t *testing.T) {
	var r Request
	if err := json.Unmarshal([]byte(`{"Name":"x","Kind":"carrier-pigeon","Payload":{}}`), &r); err == nil {
		t.Fatal("an unknown kind must not decode as HTTP")
	}
}

func TestSortPutsOtherKindsAfterHTTP(t *testing.T) {
	gql := GraphQLKind.New()
	gql.Name = "a"
	c := &Collection{Requests: []Request{gql, {Name: "z", Method: MethodOptions}, {Name: "b", Method: MethodGet}}}
	c.Sort()
	var order []string
	for _, r := range c.Requests {
		order = append(order, r.Badge()+" "+r.Name)
	}
	if want := []string{"GET b", "OPT z", "GQL a"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("sorted = %v, want %v", order, want)
	}
}

func TestPayloadSize(t *testing.T) {
	r := NewRequest()
	r.Body.Raw = "12345"
	if r.PayloadSize() != 5 {
		t.Fatalf("HTTP PayloadSize = %d", r.PayloadSize())
	}
	r.Payload = GraphQL{Query: "abc", Variables: "{}", OperationName: "Q"}
	if r.PayloadSize() != 6 {
		t.Fatalf("GraphQL PayloadSize = %d, want 6", r.PayloadSize())
	}
}
