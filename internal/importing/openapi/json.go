package openapi

import (
	"encoding/json"
	"fmt"
)

// JSON and YAML have different permitted raw string characters. Decode JSON
// directly to preserve its full character repertoire and exact numeric lexemes,
// while still rejecting duplicate keys and excessively nested documents.
func decodeJSONValue(dec *json.Decoder, depth int) (any, error) {
	if depth > 100 {
		return nil, fmt.Errorf("OpenAPI nesting exceeds 100 levels")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		out := object{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("object key must be a string")
			}
			if _, exists := out[key]; exists {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			value, err := decodeJSONValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		_, err = dec.Token()
		return out, err
	case json.Delim('['):
		out := []any{}
		for dec.More() {
			value, err := decodeJSONValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		_, err = dec.Token()
		return out, err
	default:
		return token, nil
	}
}
