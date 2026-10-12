package openapi

import (
	"encoding/json"
	"github.com/darrenburns/posting/v3/internal/model"
)

const maxMaterializedBytes = 32 << 20

// Bound aggregate request data, not just source size: a small document can reuse
// a large component arbitrarily many times. Check partially built requests too.
func (p *parser) checkRequestBudget(r *model.Request) bool {
	size := len(r.Name) + len(r.Description) + len(r.File) + len(r.URL) + len(r.Body.Raw) + len(r.Body.ContentType) + len(r.Auth.Username) + len(r.Auth.Password) + len(r.Auth.Token)
	for _, fields := range [][]model.KeyValue{r.Query, r.Headers, r.PathParams, r.Body.Form} {
		for _, field := range fields {
			size += len(field.Name) + len(field.Value)
			if size > maxMaterializedBytes-p.outputBytes {
				p.fail("OpenAPI materialized request data exceeds 32 MiB")
				return false
			}
		}
	}
	if size > maxMaterializedBytes-p.outputBytes {
		p.fail("OpenAPI materialized request data exceeds 32 MiB")
		return false
	}
	return true
}

func requestBytes(r model.Request) int {
	size := len(r.Name) + len(r.Description) + len(r.File) + len(r.URL) + len(r.Body.Raw) + len(r.Body.ContentType) + len(r.Auth.Username) + len(r.Auth.Password) + len(r.Auth.Token)
	for _, fields := range [][]model.KeyValue{r.Query, r.Headers, r.PathParams, r.Body.Form} {
		for _, field := range fields {
			size += len(field.Name) + len(field.Value)
		}
	}
	return size
}

func (p *parser) encodeJSON(value any, pretty bool) (string, bool) {
	remaining := maxMaterializedBytes - p.outputBytes
	// Traverse shared/synthesized values with a bounded counter before encoding,
	// preventing repeated schema examples from becoming a huge single body.
	estimate := remaining
	if !jsonFits(value, &estimate) {
		p.fail("OpenAPI materialized request data exceeds 32 MiB")
		return "", false
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		p.fail("invalid OpenAPI JSON example: %v", err)
		return "", false
	}
	if len(encoded) > remaining {
		p.fail("OpenAPI materialized request data exceeds 32 MiB")
		return "", false
	}
	// Source nesting is capped at 100. Keep large bodies compact instead of
	// allowing pretty-print whitespace to amplify their size by that depth.
	if pretty && len(encoded) <= remaining/202 {
		encoded, err = json.MarshalIndent(value, "", "  ")
		if err != nil {
			p.fail("invalid OpenAPI JSON example: %v", err)
			return "", false
		}
	}
	return string(encoded), true
}

// A lower bound on JSON size suffices to stop unbounded reference expansion.
// json.Marshal can add escaping, so the exact encoded length is checked too.
func jsonFits(value any, remaining *int) bool {
	if *remaining < 0 {
		return false
	}
	switch value := value.(type) {
	case object:
		*remaining -= 2
		for key, child := range value {
			*remaining -= len(key) + 4
			if !jsonFits(child, remaining) {
				return false
			}
		}
	case []any:
		*remaining -= 2
		for _, child := range value {
			*remaining--
			if !jsonFits(child, remaining) {
				return false
			}
		}
	case string:
		*remaining -= len(value) + 2
	case json.Number:
		*remaining -= len(value)
	default:
		*remaining -= 4
	}
	return *remaining >= 0
}
