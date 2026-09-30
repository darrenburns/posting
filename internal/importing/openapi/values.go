package openapi

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"strconv"
	"strings"
)

// resolve follows only JSON pointers within this document. Path Item siblings
// have undefined overlap behavior in OAS, so we diagnose and ignore them.
func (p *parser) resolve(raw any, context string) object {
	return p.resolveObject(raw, context, false)
}

func (p *parser) resolveSchema(raw any, context string) object {
	return p.resolveObject(raw, context, true)
}

func (p *parser) resolveObject(raw any, context string, allowBoolean bool) object {
	seen := map[string]bool{}
	for depth := 0; depth < 64; depth++ {
		if _, boolean := raw.(bool); boolean && allowBoolean {
			return nil
		}
		value, ok := raw.(object)
		if !ok {
			p.fail("%s: expected an object", context)
			return nil
		}
		refRaw, exists := value["$ref"]
		if !exists {
			return value
		}
		ref, ok := refRaw.(string)
		if !ok {
			p.fail("%s: $ref must be a string", context)
			return nil
		}
		if seen[ref] {
			p.fail("%s: cyclic reference %q", context, ref)
			return nil
		}
		seen[ref] = true
		for _, key := range keys(value) {
			if key != "$ref" {
				p.warn("%s: sibling %q beside $ref is not applied", context, key)
			}
		}
		if !strings.HasPrefix(ref, "#") {
			p.fail("%s: external reference %q is unsupported; bundle the document first", context, ref)
			return nil
		}
		pointer, err := url.PathUnescape(strings.TrimPrefix(ref, "#"))
		if err != nil {
			p.fail("%s: invalid reference %q", context, ref)
			return nil
		}
		if pointer != "" && !strings.HasPrefix(pointer, "/") {
			p.fail("%s: reference %q is not a JSON pointer; anchors are unsupported", context, ref)
			return nil
		}
		var target any = p.root
		if pointer != "" {
			for _, part := range strings.Split(pointer[1:], "/") {
				if invalidEscape(part) {
					p.fail("%s: invalid JSON pointer escape in %q", context, ref)
					return nil
				}
				part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
				switch container := target.(type) {
				case object:
					var found bool
					target, found = container[part]
					if !found {
						p.fail("%s: unresolved reference %q", context, ref)
						return nil
					}
				case []any:
					index, err := strconv.Atoi(part)
					if err != nil || part == "" || part[0] < '0' || part[0] > '9' || index < 0 || index >= len(container) || (len(part) > 1 && part[0] == '0') {
						p.fail("%s: invalid array reference %q", context, ref)
						return nil
					}
					target = container[index]
				default:
					p.fail("%s: unresolved reference %q", context, ref)
					return nil
				}
			}
		}
		raw = target
	}
	p.fail("%s: reference chain exceeds 64 levels", context)
	return nil
}

func invalidEscape(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			if i+1 >= len(s) || (s[i+1] != '0' && s[i+1] != '1') {
				return true
			}
			i++
		}
	}
	return false
}

// scan ensures even unused external references are diagnosed. It intentionally
// does not resolve them or reject recursive schemas that may never be needed.
func (p *parser) scan(raw any) {
	switch value := raw.(type) {
	case object:
		if ref, ok := value["$ref"].(string); ok && !strings.HasPrefix(ref, "#") {
			p.warn("external reference %q is not fetched; bundle it locally before importing affected operations", ref)
		}
		if _, exists := value["$id"]; exists {
			p.warn("JSON Schema $id resource scopes are not supported")
		}
		if _, exists := value["$dynamicRef"]; exists {
			p.warn("JSON Schema $dynamicRef is not supported")
		}
		for _, key := range keys(value) {
			p.scan(value[key])
		}
	case []any:
		for _, item := range value {
			p.scan(item)
		}
	}
}

func (p *parser) example(def object, context string) (any, bool) {
	if value, exists := def["example"]; exists {
		return value, true
	}
	if examples := obj(def["examples"]); len(examples) > 0 {
		name := keys(examples)[0]
		if len(examples) > 1 {
			p.warn("%s: selected first example %q alphabetically", context, name)
		}
		example := p.resolve(examples[name], context+" example "+name)
		if value, exists := example["value"]; exists {
			return value, true
		}
		if _, exists := example["externalValue"]; exists {
			p.warn("%s: external example is not fetched", context)
		}
	}
	if schema, exists := def["schema"]; exists {
		return p.sample(schema, context, 0)
	}
	return nil, false
}

// sample generates an editable scaffold, not an instance guaranteed to validate
// against arbitrary JSON Schema. Explicit examples always take precedence.
func (p *parser) sample(raw any, context string, depth int) (any, bool) {
	if _, ok := raw.(bool); ok {
		return nil, false
	}
	schema := p.resolveSchema(raw, context+" schema")
	for _, key := range []string{"example", "default", "const"} {
		if value, exists := schema[key]; exists {
			return value, true
		}
	}
	if examples := array(schema["examples"]); len(examples) > 0 {
		return examples[0], true
	}
	if enum := array(schema["enum"]); len(enum) > 0 {
		return enum[0], true
	}
	// The shared work budget bounds synthesis, not explicit source values.
	// An oversized earlier operation must not erase a later schema's example.
	p.budget--
	if depth > 16 || p.budget < 0 {
		p.warn("%s: recursive or oversized schema stopped while generating an example", context)
		return nil, false
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf", "$dynamicRef", "$id"} {
		if _, exists := schema[key]; exists {
			p.warn("%s: schema %s requires an explicit example; no value synthesized", context, key)
			return nil, false
		}
	}
	typ := str(schema["type"])
	if types := array(schema["type"]); typ == "" && len(types) > 0 {
		for _, candidate := range types {
			if str(candidate) != "null" {
				typ = str(candidate)
				break
			}
		}
	}
	if typ == "object" || schema["properties"] != nil {
		out := object{}
		props := obj(schema["properties"])
		for _, name := range keys(props) {
			if _, isBoolean := props[name].(bool); isBoolean {
				continue
			}
			property := p.resolveSchema(props[name], context+" property "+name)
			if boolean(property["readOnly"]) {
				continue
			}
			if value, ok := p.sample(props[name], context, depth+1); ok {
				out[name] = value
			}
		}
		p.warn("%s: generated a body/parameter scaffold from schema; review values and constraints", context)
		return out, true
	}
	if typ == "array" {
		if schema["items"] == nil {
			return nil, false
		}
		if value, ok := p.sample(schema["items"], context, depth+1); ok {
			p.warn("%s: generated a body/parameter scaffold from schema; review values and constraints", context)
			return []any{value}, true
		}
		return []any{}, true
	}
	return nil, false
}

func scalar(value any) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case nil:
		return "", true
	case bool:
		return strconv.FormatBool(value), true
	case int, int64, int32, uint, uint64, uint32, float32, float64, json.Number:
		encoded, err := json.Marshal(value)
		return string(encoded), err == nil
	}
	return "", false
}

func (p *parser) warn(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if !p.seenWarnings[message] {
		p.seenWarnings[message] = true
		p.result.Warnings = append(p.result.Warnings, message)
	}
}
func (p *parser) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}
func obj(v any) object        { value, _ := v.(object); return value }
func array(v any) []any       { value, _ := v.([]any); return value }
func str(v any) string        { value, _ := v.(string); return value }
func boolean(v any) bool      { value, _ := v.(bool); return value }
func literal(s string) string { return strings.ReplaceAll(s, "$", "$$") }
func isJSON(s string) bool {
	s = mediaType(s)
	return s == "application/json" || strings.HasSuffix(s, "+json")
}

// Media types are case insensitive, and parameters do not change the encoding.
func mediaType(s string) string {
	typ, _, err := mime.ParseMediaType(s)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(s, ";")[0]))
	}
	return typ
}
