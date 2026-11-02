package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/theory/jsonpath"
)

// Capture paths are JSONPath expressions with an optional root prefix.
// Shorthand only adds the prefix; both forms use the same parser.
func capturePath(path string) (*jsonpath.Path, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	switch first := path[0]; {
	case first == '$':
		// Explicit JSONPath is parsed as written, without a shorthand fallback.
	case first == '[':
		path = "$" + path
	case first == '_' || first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' || first >= 0x80:
		path = "$." + path
	default:
		return nil, fmt.Errorf("invalid path %q: start with $, [ or a member name", path)
	}
	return jsonpath.Parse(path)
}

// ValidateCaptures rejects unusable variable names and invalid JSONPath
// before a request file is accepted or saved.
func ValidateCaptures(captures map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(captures)) {
		if !isIdentifier(name) {
			return fmt.Errorf("capture %q: invalid variable name", name)
		}
		if _, err := capturePath(captures[name]); err != nil {
			return fmt.Errorf("capture %s: %w", name, err)
		}
	}
	return nil
}

// CaptureValues extracts all captures or returns an error with no values.
// Strings are unquoted; all other values become compact JSON. Numbers keep
// their original precision, including IDs larger than a float64 can hold.
func CaptureValues(captures map[string]string, body []byte) (map[string]string, error) {
	if len(captures) == 0 {
		return nil, nil
	}
	if err := ValidateCaptures(captures); err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("captures require a valid JSON response body")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(captures))
	for _, name := range slices.Sorted(maps.Keys(captures)) {
		path := captures[name]
		query, _ := capturePath(path) // Validated above.
		matches := query.Select(root)
		if len(matches) != 1 {
			return nil, fmt.Errorf("capture %s: path %q matched %d values; expected exactly one", name, path, len(matches))
		}
		value := matches[0]
		if text, ok := value.(string); ok {
			values[name] = text
		} else {
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("capture %s: %w", name, err)
			}
			values[name] = string(encoded)
		}
	}
	return values, nil
}
