package collection

import (
	"fmt"

	"github.com/darrenburns/posting/internal/model"
)

// graphQLFile is a GraphQL request's `graphql:` block.
type graphQLFile struct {
	Query         yamlString `yaml:"query,omitempty"`
	Variables     yamlString `yaml:"variables,omitempty"`
	OperationName yamlString `yaml:"operation_name,omitempty"`
}

// decodePayload reads the file's kind and that kind's block. A file whose
// keys don't fit its kind is an error, so nothing in it is silently dropped.
func decodePayload(in requestFile) (model.Payload, error) {
	id := model.KindHTTP
	if in.Kind != "" {
		id = model.KindID(in.Kind)
	}
	kind, ok := model.KindByID(id)
	if !ok {
		return nil, fmt.Errorf("unsupported request kind %q", in.Kind)
	}
	if err := checkFields(in, kind); err != nil {
		return nil, err
	}
	if in.GraphQL != nil && kind.ID != model.KindGraphQL {
		return nil, fmt.Errorf("a graphql block needs kind: graphql")
	}
	switch kind.ID {
	case model.KindHTTP:
		return nil, nil
	case model.KindGraphQL:
		var g model.GraphQL
		if f := in.GraphQL; f != nil {
			g = model.GraphQL{Query: string(f.Query), Variables: string(f.Variables), OperationName: string(f.OperationName)}
		}
		return g, nil
	}
	return nil, fmt.Errorf("no file format for %s requests", kind.Label)
}

// checkFields rejects keys for shared fields the kind doesn't use.
func checkFields(in requestFile, kind *model.Kind) error {
	for _, key := range []struct {
		name    string
		field   model.Fields
		present bool
	}{
		{"method", model.FieldMethod, in.Method != ""},
		{"body", model.FieldBody, in.Body != nil},
		{"params", model.FieldQuery, in.Params != nil},
		{"path_params", model.FieldPathParams, in.PathParams != nil},
		{"headers", model.FieldHeaders, in.Headers != nil},
		{"auth", model.FieldAuth, in.Auth != nil},
	} {
		if key.present && !kind.Fields.Has(key.field) {
			return fmt.Errorf("%s requests don't have a %s", kind.Label, key.name)
		}
	}
	return nil
}

// encodePayload writes the kind and its block for requests that aren't HTTP.
func encodePayload(req model.Request, out *requestFile) error {
	switch p := req.Payload.(type) {
	case nil:
	case model.GraphQL:
		out.Kind = string(model.KindGraphQL)
		out.GraphQL = &graphQLFile{Query: yamlString(p.Query), Variables: yamlString(p.Variables), OperationName: yamlString(p.OperationName)}
	default:
		return fmt.Errorf("no file format for %s requests", req.Kind().Label)
	}
	return nil
}
