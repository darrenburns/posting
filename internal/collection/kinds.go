package collection

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/darrenburns/posting/internal/model"
)

// graphQLFile is a GraphQL request's `graphql:` block.
type graphQLFile struct {
	Query         yamlString `yaml:"query,omitempty"`
	Variables     yamlString `yaml:"variables,omitempty"`
	OperationName yamlString `yaml:"operation_name,omitempty"`
}

func (f *graphQLFile) UnmarshalYAML(node *yaml.Node) error {
	type plain graphQLFile
	return decodeStrict(node, "graphql", (*plain)(f))
}

// kindBlocks are the kinds' own blocks in a request file: the key, whether
// the file has it, and the payload it holds.
var kindBlocks = map[model.KindID]struct {
	key     string
	present func(requestFile) bool
	decode  func(requestFile) model.Payload
}{
	model.KindGraphQL: {
		key:     "graphql",
		present: func(in requestFile) bool { return in.GraphQL != nil },
		decode: func(in requestFile) model.Payload {
			var g model.GraphQL
			if f := in.GraphQL; f != nil {
				g = model.GraphQL{Query: string(f.Query), Variables: string(f.Variables), OperationName: string(f.OperationName)}
			}
			return g
		},
	},
}

// decodePayload reads the file's kind and that kind's block. A file whose
// keys don't fit its kind is an error, so nothing in it is silently dropped.
func decodePayload(in requestFile) (model.Payload, error) {
	id := model.KindHTTP
	if in.Kind != "" {
		id = model.KindID(strings.ToLower(in.Kind))
	}
	kind, ok := model.KindByID(id)
	if !ok {
		return nil, fmt.Errorf("unsupported request kind %q", in.Kind)
	}
	if err := checkFields(in, kind); err != nil {
		return nil, err
	}
	for other, block := range kindBlocks {
		if other != kind.ID && block.present(in) {
			return nil, fmt.Errorf("a %s block needs kind: %s", block.key, other)
		}
	}
	if kind == model.HTTPKind {
		return nil, nil
	}
	block, ok := kindBlocks[kind.ID]
	if !ok {
		return nil, fmt.Errorf("no file format for %s requests", kind.Label)
	}
	return block.decode(in), nil
}

// decodeStrict decodes a kind's block into out, a key it doesn't have being
// an error that names it. Posting 2 never wrote these blocks, so a key out
// of place is a mistake, which saving would otherwise erase.
func decodeStrict(node *yaml.Node, name string, out any) error {
	if node.Kind == yaml.MappingNode {
		known := map[string]bool{}
		fields := reflect.TypeOf(out).Elem()
		for i := range fields.NumField() {
			key, _, _ := strings.Cut(fields.Field(i).Tag.Get("yaml"), ",")
			known[key] = true
		}
		for i := 0; i < len(node.Content); i += 2 {
			if key := node.Content[i]; !known[key.Value] {
				return fmt.Errorf("line %d: %s has no key %q", key.Line, name, key.Value)
			}
		}
	}
	return node.Decode(out)
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
