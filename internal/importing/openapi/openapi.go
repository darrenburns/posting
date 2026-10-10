// Package openapi converts OpenAPI 3.0 and 3.1 documents to editable requests.
// It never fetches references or executes examples.
package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/darrenburns/posting/v3/internal/importing"
	"github.com/darrenburns/posting/v3/internal/model"
	"gopkg.in/yaml.v3"
)

type object = map[string]any

type parser struct {
	root          object
	result        importing.Result
	err           error
	seenWarnings  map[string]bool
	variableNames map[string]bool
	budget        int
	outputBytes   int
}

// Parse accepts one JSON or YAML document. Conversion warnings identify lossy
// features; errors identify unsupported versions, malformed data or references
// needed to construct requests. No partial result is returned on error.
func Parse(data []byte) (importing.Result, error) {
	if len(data) > 32<<20 {
		return importing.Result{}, fmt.Errorf("OpenAPI document exceeds 32 MiB")
	}
	var raw any
	var document yaml.Node
	jsonInput := json.Valid(data)
	if jsonInput {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var err error
		raw, err = decodeJSONValue(dec, 0)
		if err != nil {
			return importing.Result{}, fmt.Errorf("decode OpenAPI JSON: %w", err)
		}
	} else {
		dec := yaml.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&document); err != nil {
			return importing.Result{}, fmt.Errorf("decode OpenAPI: %w", err)
		}
		if err := prepareYAML(&document, 0); err != nil {
			return importing.Result{}, err
		}
		if err := document.Decode(&raw); err != nil {
			return importing.Result{}, fmt.Errorf("decode OpenAPI: %w", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return importing.Result{}, fmt.Errorf("OpenAPI input must contain exactly one document")
		}
	}
	normalized, err := normalize(raw, 0)
	if err != nil {
		return importing.Result{}, err
	}
	if !jsonInput {
		budget := 250000
		normalized, err = exactYAMLNumbers(&document, normalized, &budget)
		if err != nil {
			return importing.Result{}, err
		}
	}
	root, ok := normalized.(object)
	if !ok {
		return importing.Result{}, fmt.Errorf("OpenAPI document must be an object")
	}
	version := str(root["openapi"])
	if !regexp.MustCompile(`^3\.[01]\.[0-9]+$`).MatchString(version) {
		return importing.Result{}, fmt.Errorf("unsupported OpenAPI version %q (supported: 3.0.x and 3.1.x; Swagger 2 is not supported)", version)
	}
	p := &parser{root: root, seenWarnings: map[string]bool{}, variableNames: map[string]bool{}, budget: 10000}
	p.result.Name = str(obj(root["info"])["title"])
	if p.result.Name == "" {
		p.result.Name = "OpenAPI"
	}
	p.outputBytes = len(p.result.Name)
	p.scan(root)
	paths, ok := root["paths"].(object)
	if !ok {
		return importing.Result{}, fmt.Errorf("OpenAPI paths must be an object")
	}
	for _, path := range keys(paths) {
		if strings.HasPrefix(path, "x-") {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			return importing.Result{}, fmt.Errorf("OpenAPI path %q must start with /", path)
		}
		item := p.resolve(paths[path], "path "+path)
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
			rawOp, exists := item[method]
			if !exists {
				continue
			}
			if method == "trace" {
				p.warn("%s: TRACE is not supported by Posting; operation skipped", path)
				continue
			}
			op, valid := rawOp.(object)
			if !valid {
				p.fail("%s %s: operation must be an object", method, path)
				continue
			}
			p.operation(path, method, item, op)
			if p.err != nil {
				return importing.Result{}, p.err
			}
		}
	}
	if p.err != nil {
		return importing.Result{}, p.err
	}
	return p.result, nil
}

// OpenAPI uses JSON-compatible YAML: dates are strings, not time.Time values.
// Check depth before decoding and before subsequent recursive tree walks.
func prepareYAML(node *yaml.Node, depth int) error {
	if depth > 100 {
		return fmt.Errorf("OpenAPI nesting exceeds 100 levels")
	}
	if node.Tag == "!!timestamp" {
		node.Tag = "!!str"
	}
	for _, child := range node.Content {
		if err := prepareYAML(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func normalize(v any, depth int) (any, error) {
	if depth > 100 {
		return nil, fmt.Errorf("OpenAPI nesting exceeds 100 levels")
	}
	switch x := v.(type) {
	case map[string]any:
		out := object{}
		for k, v := range x {
			n, e := normalize(v, depth+1)
			if e != nil {
				return nil, e
			}
			out[k] = n
		}
		return out, nil
	case map[any]any:
		out := object{}
		for k, v := range x {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("OpenAPI object keys must be strings")
			}
			n, e := normalize(v, depth+1)
			if e != nil {
				return nil, e
			}
			out[key] = n
		}
		return out, nil
	case []any:
		for i, v := range x {
			n, e := normalize(v, depth+1)
			if e != nil {
				return nil, e
			}
			x[i] = n
		}
		return x, nil
	default:
		return v, nil
	}
}

func (p *parser) operation(path, method string, item, op object) {
	context := strings.ToUpper(method) + " " + path
	r := model.NewRequest()
	r.Method = model.Method(strings.ToUpper(method))
	r.Name = str(op["summary"])
	if r.Name == "" {
		r.Name = str(op["operationId"])
	}
	if r.Name == "" {
		r.Name = context
	}
	r.Description = str(op["description"])
	if r.Description == "" {
		r.Description = str(item["description"])
	}
	r.File = r.Name + ".posting.yaml"
	if tags := array(op["tags"]); len(tags) > 0 && str(tags[0]) != "" {
		r.File = str(tags[0]) + "/" + r.File
	}
	servers, exists := op["servers"]
	if !exists {
		servers, exists = item["servers"]
	}
	if !exists {
		servers = p.root["servers"]
	}
	base := p.server(servers, context)
	r.URL = literal(strings.ReplaceAll(strings.TrimRight(base, "/"), "/:", "/::")) + escapePath(path)
	// BASE_URL is introduced by the importer, not a literal source string.
	if strings.HasPrefix(base, "${BASE_URL}") {
		r.URL = "${BASE_URL}" + literal(strings.ReplaceAll(strings.TrimRight(strings.TrimPrefix(base, "${BASE_URL}"), "/"), "/:", "/::")) + escapePath(path)
	}
	params := p.parameters(item["parameters"], context)
	override := p.parameters(op["parameters"], context)
	for _, parameter := range override {
		replaced := false
		for i, previous := range params {
			if str(previous["name"]) == str(parameter["name"]) && str(previous["in"]) == str(parameter["in"]) {
				params[i] = parameter
				replaced = true
				break
			}
		}
		if !replaced {
			params = append(params, parameter)
		}
	}
	for _, parameter := range params {
		p.parameter(&r, parameter, context)
		if p.err != nil || !p.checkRequestBudget(&r) {
			return
		}
	}
	for _, name := range model.PathParamNames(r.URL) {
		found := false
		for _, parameter := range r.PathParams {
			if parameter.Name == name {
				found = true
				break
			}
		}
		if !found {
			p.warn("%s: path parameter %q has no supported definition; value needs editing", context, name)
		}
	}
	if strings.Contains(r.URL, "{") && strings.Contains(r.URL, "}") {
		// Posting's path slots work only when a placeholder occupies a segment.
		for _, segment := range strings.Split(path, "/") {
			if strings.Contains(segment, "{") && !(strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && strings.Count(segment, "{") == 1) {
				p.warn("%s: embedded path template %q needs manual editing", context, segment)
			}
		}
	}
	if body, exists := op["requestBody"]; exists {
		p.body(&r, p.resolve(body, context+" requestBody"), context)
	}
	security, exists := op["security"]
	if !exists {
		security = p.root["security"]
	}
	if p.err != nil || !p.checkRequestBudget(&r) {
		return
	}
	p.security(&r, security, context)
	if len(obj(op["callbacks"])) > 0 {
		p.warn("%s: callbacks are not imported", context)
	}
	if p.err != nil || !p.checkRequestBudget(&r) {
		return
	}
	p.outputBytes += requestBytes(r)
	p.result.Requests = append(p.result.Requests, r)
}

func (p *parser) server(raw any, context string) string {
	servers := array(raw)
	if raw != nil && servers == nil {
		p.fail("%s: servers must be an array", context)
	}
	if len(servers) == 0 {
		p.addVariable("BASE_URL", "")
		p.warn("relative server URLs require setting BASE_URL to the API origin before sending")
		return "${BASE_URL}"
	}
	if len(servers) > 1 {
		p.warn("%s: selected first of %d servers", context, len(servers))
	}
	server, ok := servers[0].(object)
	if !ok {
		p.fail("%s: server must be an object", context)
		return ""
	}
	value := str(server["url"])
	if value == "" {
		p.fail("%s: server URL is required", context)
		return ""
	}
	variables := obj(server["variables"])
	value = template.ReplaceAllStringFunc(value, func(token string) string {
		name := token[1 : len(token)-1]
		def := obj(variables[name])
		v, exists := def["default"]
		if !exists {
			p.fail("%s: server variable %q has no default", context, name)
			return token
		}
		s, ok := v.(string)
		if !ok {
			p.fail("%s: server variable %q default must be a string", context, name)
		}
		return s
	})
	if strings.ContainsAny(value, "?#") {
		p.fail("%s: server URLs containing query strings or fragments are not supported", context)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		p.fail("%s: invalid server URL: %v", context, err)
		return value
	}
	if parsed.Scheme == "" {
		if parsed.Host != "" {
			p.fail("%s: scheme-relative server URL requires choosing an explicit scheme", context)
			return value
		}
		p.addVariable("BASE_URL", "")
		p.warn("relative server URLs require setting BASE_URL to the API origin before sending")
		return "${BASE_URL}/" + strings.TrimLeft(value, "/")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		p.fail("%s: server scheme %q is not supported", context, parsed.Scheme)
	}
	if parsed.Host == "" {
		p.fail("%s: server URL has no host", context)
	}
	return value
}

var template = regexp.MustCompile(`\{([^{}]+)\}`)

func escapePath(path string) string {
	segments := strings.Split(literal(path), "/")
	for i, s := range segments {
		if strings.HasPrefix(s, ":") {
			segments[i] = ":" + s
		}
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && strings.Count(s, "{") == 1 {
			segments[i] = ":" + s[1:len(s)-1]
		}
	}
	return strings.Join(segments, "/")
}

func (p *parser) parameters(raw any, context string) []object {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		p.fail("%s: parameters must be an array", context)
		return nil
	}
	var out []object
	seen := map[string]bool{}
	for _, value := range list {
		v := p.resolve(value, context+" parameter")
		name, in := str(v["name"]), str(v["in"])
		if name == "" || in == "" {
			p.fail("%s: parameter name and in are required", context)
			continue
		}
		key := in + "\x00" + name
		if seen[key] {
			p.fail("%s: duplicate %s parameter %q", context, in, name)
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func (p *parser) parameter(r *model.Request, parameter object, context string) {
	name, in := str(parameter["name"]), str(parameter["in"])
	context += " parameter " + name
	if in == "header" {
		switch strings.ToLower(name) {
		case "accept", "content-type", "authorization":
			return
		}
	}
	style := str(parameter["style"])
	if style == "" {
		if in == "query" || in == "cookie" {
			style = "form"
		} else {
			style = "simple"
		}
	}
	explode := style == "form"
	if v, ok := parameter["explode"].(bool); ok {
		explode = v
	}
	if boolean(parameter["allowReserved"]) {
		p.warn("%s: allowReserved cannot be preserved; parameter omitted", context)
		return
	}
	var value any
	var present bool
	if content := obj(parameter["content"]); len(content) > 0 {
		if len(content) != 1 {
			p.fail("%s: content must have exactly one media type", context)
			return
		}
		media := keys(content)[0]
		v, found := p.example(obj(content[media]), context)
		if !found {
			p.warn("%s: content parameter has no usable example; omitted", context)
			return
		}
		if isJSON(media) {
			text, ok := p.encodeJSON(v, false)
			if !ok {
				return
			}
			value = text
			present = true
		} else if s, ok := v.(string); ok {
			value = s
			present = true
		} else {
			p.warn("%s: unsupported content parameter encoding; omitted", context)
			return
		}
	} else {
		value, present = p.example(parameter, context)
	}
	if !present {
		value = ""
		p.warn("%s: no example or default; value needs editing", context)
	}
	switch in {
	case "query":
		values, ok := parameterValues(name, value, style, explode)
		if !ok {
			p.warn("%s: unsupported %s serialization; parameter omitted", context, style)
			return
		}
		for _, v := range values {
			v.Name = literal(v.Name)
			v.Value = literal(v.Value)
			v.Enabled = present || boolean(parameter["required"])
			r.Query = append(r.Query, v)
		}
	case "header":
		if style != "simple" {
			p.warn("%s: unsupported header style %s; omitted", context, style)
			return
		}
		s, ok := scalar(value)
		if !ok {
			p.warn("%s: complex header serialization is not supported; omitted", context)
			return
		}
		r.Headers = append(r.Headers, model.KeyValue{Name: literal(name), Value: literal(s), Enabled: present || boolean(parameter["required"])})
	case "path":
		if style != "simple" {
			p.warn("%s: unsupported path style %s; value needs editing", context, style)
			return
		}
		s, ok := scalar(value)
		if !ok {
			p.warn("%s: complex path serialization is not supported; value needs editing", context)
			return
		}
		r.PathParams = append(r.PathParams, model.KeyValue{Name: name, Value: literal(s), Enabled: true})
	case "cookie":
		s, ok := scalar(value)
		if !ok || style != "form" {
			p.warn("%s: complex cookie serialization is not supported; omitted", context)
			return
		}
		if present || boolean(parameter["required"]) {
			p.cookie(r, name, s)
		}
	default:
		p.fail("%s: unsupported parameter location %q", context, in)
	}
}

// parameterValues deliberately supports only encodings Posting can round-trip
// without losing the distinction between structural and value delimiters.
func parameterValues(name string, value any, style string, explode bool) ([]model.KeyValue, bool) {
	if s, ok := scalar(value); ok {
		if style != "form" {
			return nil, false
		}
		return []model.KeyValue{{Name: name, Value: s, Enabled: true}}, true
	}
	switch v := value.(type) {
	case []any:
		if style != "form" || !explode {
			return nil, false
		}
		out := []model.KeyValue{}
		for _, item := range v {
			s, ok := scalar(item)
			if !ok {
				return nil, false
			}
			out = append(out, model.KeyValue{Name: name, Value: s, Enabled: true})
		}
		return out, true
	case object:
		if !(style == "form" && explode) && style != "deepObject" {
			return nil, false
		}
		out := []model.KeyValue{}
		for _, key := range keys(v) {
			s, ok := scalar(v[key])
			if !ok {
				return nil, false
			}
			field := key
			if style == "deepObject" {
				field = name + "[" + key + "]"
			}
			out = append(out, model.KeyValue{Name: field, Value: s, Enabled: true})
		}
		return out, true
	}
	return nil, false
}

func (p *parser) body(r *model.Request, body object, context string) {
	content := obj(body["content"])
	if len(content) == 0 {
		p.warn("%s: request body has no supported content", context)
		return
	}
	media := keys(content)[0]
	// Favor a concrete, common representation when alternatives exist.
	for _, candidate := range keys(content) {
		if isJSON(candidate) {
			media = candidate
			break
		}
	}
	if _, exists := content["application/json"]; exists {
		media = "application/json"
	} else if !isJSON(media) {
		if _, exists := content["application/x-www-form-urlencoded"]; exists {
			media = "application/x-www-form-urlencoded"
		}
	}
	if len(content) > 1 {
		p.warn("%s: selected %s from %d request body media types", context, media, len(content))
	}
	definition := obj(content[media])
	value, present := p.example(definition, context+" body")
	if strings.HasPrefix(mediaType(media), "multipart/") {
		p.warn("%s: multipart bodies are not supported; body omitted", context)
		return
	}
	if !present {
		p.warn("%s: request body has no usable example or default; body needs editing", context)
	}
	r.Options.SubstituteBodyVariables = false // Examples contain literal source bytes, including '$'.
	if mediaType(media) == "application/x-www-form-urlencoded" {
		values, ok := value.(object)
		if !ok {
			p.warn("%s: form body example must be an object; body omitted", context)
			return
		}
		r.Body = model.Body{Type: model.BodyForm, ContentType: "application/x-www-form-urlencoded", Form: []model.KeyValue{}}
		if media != r.Body.ContentType {
			r.Headers = append(r.Headers, model.KeyValue{Name: "Content-Type", Value: literal(media), Enabled: true})
		}
		encoding := obj(definition["encoding"])
		for _, name := range keys(values) {
			fields, ok := p.formValues(name, values[name], obj(encoding[name]))
			if !ok {
				p.warn("%s: form field %s has unsupported serialization; omitted", context, name)
				continue
			}
			r.Body.Form = append(r.Body.Form, fields...)
			if p.err != nil || !p.checkRequestBudget(r) {
				return
			}
		}
		return
	}
	raw := ""
	if present {
		if isJSON(media) {
			var ok bool
			raw, ok = p.encodeJSON(value, true)
			if !ok {
				return
			}
		} else {
			var ok bool
			raw, ok = value.(string)
			if !ok {
				p.warn("%s: %s body requires a string example; body omitted", context, media)
				return
			}
		}
	}
	if strings.Contains(media, "*") {
		p.warn("%s: wildcard body media type %s needs manual selection", context, media)
	}
	r.Body = model.Body{Type: model.BodyRaw, ContentType: media, Raw: raw}
}

// Explicit style/explode/allowReserved select parameter serialization. Without
// them, object fields use JSON content, including objects in repeated arrays.
// Per OAS, headers are ignored for URL-encoded forms, and contentType is ignored
// when parameter serialization is selected.
func (p *parser) formValues(name string, value any, encoding object) ([]model.KeyValue, bool) {
	_, styleSet := encoding["style"]
	_, explodeSet := encoding["explode"]
	_, reservedSet := encoding["allowReserved"]
	if styleSet || explodeSet || reservedSet {
		if boolean(encoding["allowReserved"]) {
			return nil, false
		}
		style := str(encoding["style"])
		if style == "" {
			style = "form"
		}
		explode := style == "form"
		if v, ok := encoding["explode"].(bool); ok {
			explode = v
		}
		return parameterValues(name, value, style, explode)
	}
	values, array := value.([]any)
	if !array {
		values = []any{value}
	}
	out := []model.KeyValue{}
	for _, value := range values {
		contentType := str(encoding["contentType"])
		_, objectValue := value.(object)
		if contentType == "" && objectValue {
			contentType = "application/json"
		}
		var text string
		if isJSON(contentType) {
			var ok bool
			text, ok = p.encodeJSON(value, false)
			if !ok {
				return nil, false
			}
		} else {
			if contentType != "" && mediaType(contentType) != "text/plain" {
				return nil, false
			}
			var ok bool
			text, ok = scalar(value)
			if !ok {
				return nil, false
			}
		}
		out = append(out, model.KeyValue{Name: name, Value: text, Enabled: true})
	}
	return out, true
}
