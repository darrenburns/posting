// Package postman converts Postman v2 and v2.1 JSON collections without running
// scripts, reading referenced files, or making network requests.
package postman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

type variable struct {
	Key      string
	ID       string
	Value    json.RawMessage
	Disabled bool
}
type pair struct {
	Key      string
	Value    *string
	Disabled bool
}
type node struct {
	Name                    string
	Item                    []node
	Request                 json.RawMessage
	Auth                    json.RawMessage
	Variable                []variable
	Event                   []json.RawMessage
	ProtocolProfileBehavior json.RawMessage
}
type request struct {
	Method      string
	URL         json.RawMessage
	Header      json.RawMessage
	Description json.RawMessage
	Auth        json.RawMessage
	Body        *struct {
		Mode       string
		Raw        string
		URLEncoded []pair `json:"urlencoded"`
		Disabled   bool
		Options    struct{ Raw struct{ Language string } }
	}
	Proxy       json.RawMessage
	Certificate json.RawMessage
}
type parser struct {
	result         importing.Result
	seen           map[string]bool
	defaults       map[string]string
	expansionBytes int
	err            error
}

// Parse converts a collection into requests and collection defaults. A fatal
// structural error returns no partial result. Unsupported optional features are
// reported through Warnings, alongside the requests that need attention.
func Parse(data []byte) (importing.Result, error) {
	var in struct {
		Info *struct {
			Name   string
			Schema string
		}
		Item                    []node
		Auth                    json.RawMessage
		Variable                []variable
		Event                   []json.RawMessage
		ProtocolProfileBehavior json.RawMessage
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return importing.Result{}, fmt.Errorf("postman: invalid collection JSON: %w", err)
	}
	if in.Info == nil || in.Item == nil {
		return importing.Result{}, fmt.Errorf("postman: expected a v2/v2.1 collection with info and item array")
	}
	if in.Info.Schema != "" && !strings.Contains(in.Info.Schema, "/v2.0.0/") && !strings.Contains(in.Info.Schema, "/v2.1.0/") {
		return importing.Result{}, fmt.Errorf("postman: unsupported collection schema %q (expected v2.0 or v2.1)", in.Info.Schema)
	}
	p := &parser{result: importing.Result{Name: in.Info.Name}, seen: map[string]bool{}, defaults: map[string]string{}}
	for _, v := range in.Variable {
		if !v.Disabled {
			p.defaults[v.name()] = scalar(v.Value)
		}
	}
	// Environment files contain literal values. Flatten references between
	// exported defaults now because Posting deliberately substitutes only once.
	added := map[string]bool{}
	for _, v := range in.Variable {
		name := v.name()
		if v.Disabled || added[name] {
			continue
		}
		added[name] = true
		if !identifier.MatchString(name) {
			p.warn("collection", "variable %q cannot be represented as a Posting variable; references remain literal", name)
			continue
		}
		value := p.expand(p.defaults[name], p.defaults, map[string]bool{name: true}, "collection")
		if reference.MatchString(value) {
			p.warn("collection", "variable %q contains unresolved Postman references; provide a concrete value in the imported environment", name)
		}
		p.result.Variables = append(p.result.Variables, model.Variable{Name: name, Value: strings.ReplaceAll(value, "$", "$$"), Source: "postman"})
	}
	if len(in.Event) > 0 {
		p.warn("collection", "scripts and tests are not imported or executed")
	}
	p.profile("collection", in.ProtocolProfileBehavior)
	if err := p.walk(in.Item, nil, in.Auth, map[string]string{}, 0); err != nil {
		return importing.Result{}, err
	}
	if p.err != nil {
		return importing.Result{}, p.err
	}
	if len(p.result.Requests) == 0 {
		return importing.Result{}, fmt.Errorf("postman: collection contains no requests")
	}
	return p.result, nil
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var reference = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func (v variable) name() string {
	if v.Key != "" {
		return v.Key
	}
	return v.ID
}
func scalar(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	return string(raw)
}
func present(raw json.RawMessage) bool {
	return len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
func (p *parser) warn(where, format string, args ...any) {
	s := where + ": " + fmt.Sprintf(format, args...)
	if !p.seen[s] {
		p.seen[s] = true
		p.result.Warnings = append(p.result.Warnings, s)
	}
}
func (p *parser) profile(where string, raw json.RawMessage) {
	if present(raw) && string(raw) != "{}" {
		p.warn(where, "Postman protocol settings are not imported; Posting defaults apply")
	}
}
func (p *parser) walk(items []node, dirs []string, auth json.RawMessage, local map[string]string, depth int) error {
	if depth > 128 {
		return fmt.Errorf("postman: folder nesting exceeds 128 levels")
	}
	for index, item := range items {
		if p.err != nil {
			return p.err
		}
		name := item.Name
		if name == "" {
			name = fmt.Sprintf("Untitled %d", index+1)
		}
		parts := append(append([]string{}, dirs...), name)
		where := strings.Join(parts, "/")
		scope := make(map[string]string, len(local)+len(item.Variable))
		for k, v := range local {
			scope[k] = v
		}
		for _, v := range item.Variable {
			if !v.Disabled {
				scope[v.name()] = scalar(v.Value)
			}
		}
		inherited := auth
		if present(item.Auth) {
			inherited = item.Auth
		}
		if len(item.Event) > 0 {
			p.warn(where, "scripts and tests are not imported or executed")
		}
		p.profile(where, item.ProtocolProfileBehavior)
		if item.Item != nil {
			if present(item.Request) {
				return fmt.Errorf("postman: %s has both request and child items", where)
			}
			if err := p.walk(item.Item, parts, inherited, scope, depth+1); err != nil {
				return err
			}
			continue
		}
		if !present(item.Request) {
			return fmt.Errorf("postman: %s has neither a request nor a folder item array", where)
		}
		req, err := p.convert(item.Request, inherited, scope, where)
		if err != nil {
			return fmt.Errorf("postman: %s: %w", where, err)
		}
		req.Name = name
		// Leave sanitizing individual components to the common writer; never clean
		// '..' here because that would silently discard a source folder.
		req.File = strings.Join(parts, "/") + ".posting.yaml"
		p.result.Requests = append(p.result.Requests, req)
	}
	return nil
}

func (p *parser) convert(raw, inherited json.RawMessage, scope map[string]string, where string) (model.Request, error) {
	req := model.NewRequest()
	var in request
	var shorthand string
	if json.Unmarshal(raw, &shorthand) == nil {
		in.URL, _ = json.Marshal(shorthand)
	} else if err := json.Unmarshal(raw, &in); err != nil {
		return req, fmt.Errorf("invalid request: %w", err)
	}
	if in.Method != "" {
		req.Method = model.Method(strings.ToUpper(in.Method))
	}
	supported := false
	for _, m := range model.Methods {
		if m == req.Method {
			supported = true
		}
	}
	if !supported {
		return req, fmt.Errorf("HTTP method %q is not supported by Posting", in.Method)
	}
	scope = p.scopedAliases(scope)
	transform := func(s string) string { return p.template(s, scope, where) }
	if err := p.readURL(&req, in.URL, transform, where); err != nil {
		return req, err
	}
	if strings.TrimSpace(req.URL) == "" {
		return req, fmt.Errorf("request URL is empty")
	}
	var desc string
	if json.Unmarshal(in.Description, &desc) != nil {
		var d struct{ Content string }
		_ = json.Unmarshal(in.Description, &d)
		desc = d.Content
	}
	req.Description = desc
	if present(in.Header) {
		var headers []pair
		if err := json.Unmarshal(in.Header, &headers); err != nil {
			var block string
			if json.Unmarshal(in.Header, &block) != nil {
				return req, fmt.Errorf("headers must be an array or string")
			}
			for _, line := range strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				key, value, ok := strings.Cut(line, ":")
				if !ok {
					return req, fmt.Errorf("invalid header line %q", line)
				}
				value = strings.TrimSpace(value)
				headers = append(headers, pair{Key: strings.TrimSpace(key), Value: &value})
			}
		}
		req.Headers = convertPairs(headers, transform, false)
	}
	if in.Body != nil && !in.Body.Disabled {
		switch in.Body.Mode {
		case "", "raw":
			req.Body = model.Body{Type: model.BodyRaw, Raw: transform(in.Body.Raw), ContentType: "text/plain"}
			switch in.Body.Options.Raw.Language {
			case "json":
				req.Body.ContentType = "application/json"
			case "xml":
				req.Body.ContentType = "application/xml"
			case "html":
				req.Body.ContentType = "text/html"
			case "javascript":
				req.Body.ContentType = "application/javascript"
			}
			for _, h := range req.Headers {
				if h.Enabled && strings.EqualFold(h.Name, "Content-Type") {
					req.Body.ContentType = h.Value
					break
				}
			}
		case "urlencoded":
			req.Body = model.Body{Type: model.BodyForm, Form: convertPairs(in.Body.URLEncoded, transform, false), ContentType: "application/x-www-form-urlencoded"}
		default:
			p.warn(where, "body mode %q is unsupported; body omitted (multipart/files and GraphQL require manual conversion)", in.Body.Mode)
		}
	}
	a := inherited
	if present(in.Auth) {
		a = in.Auth
	}
	if err := p.readAuth(&req, a, transform, where); err != nil {
		return req, err
	}
	for _, q := range req.Query {
		for _, field := range []string{q.Name, q.Value} {
			for _, ref := range model.FindVariables(field) {
				value, ok := p.defaults[ref.Name]
				if ok && (strings.ContainsAny(value, "%+") || reference.MatchString(value)) {
					p.warn(where, "encoded query variable %q may need a decoded value in Posting; percent escapes and plus signs in substituted values are encoded literally", ref.Name)
				}
			}
		}
	}
	if present(in.Proxy) {
		p.warn(where, "proxy configuration is not imported")
	}
	if present(in.Certificate) {
		p.warn(where, "client certificate configuration is not imported")
	}
	return req, nil
}

func convertPairs(in []pair, transform func(string) string, encoded bool) []model.KeyValue {
	var out []model.KeyValue
	for _, v := range in {
		value := ""
		if v.Value != nil {
			value = *v.Value
		}
		key := v.Key
		key, value = transform(key), transform(value)
		if encoded {
			key = decodeQuery(escapeEncodedDollars(key))
			value = decodeQuery(escapeEncodedDollars(value))
		}
		out = append(out, model.KeyValue{Name: key, Value: value, Enabled: !v.Disabled})
	}
	return out
}
func decodeQuery(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

func (p *parser) readURL(req *model.Request, raw json.RawMessage, transform func(string) string, where string) error {
	var text string
	var vars []variable
	var query []pair
	structuredQuery := false
	if json.Unmarshal(raw, &text) != nil {
		var u struct {
			Raw, Protocol, Port, Hash string
			Host, Path                json.RawMessage
			Query                     json.RawMessage
			Variable                  []variable
			Auth                      *struct{ User, Password string }
		}
		if !present(raw) {
			return fmt.Errorf("request URL is missing")
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		text = u.Raw
		// URL components are authoritative when supplied. raw-only objects remain
		// accepted because many third-party collection exporters produce them.
		if present(u.Host) {
			host, err := joinURLParts(u.Host, ".")
			if err != nil {
				return fmt.Errorf("invalid URL host: %w", err)
			}
			text = host
			if u.Auth != nil {
				text = u.Auth.User + ":" + u.Auth.Password + "@" + text
			}
			if u.Protocol != "" {
				text = strings.TrimSuffix(u.Protocol, "://") + "://" + text
			}
			if u.Port != "" {
				text += ":" + u.Port
			}
			if present(u.Path) {
				path, err := joinURLParts(u.Path, "/")
				if err != nil {
					return fmt.Errorf("invalid URL path: %w", err)
				}
				if bytes.HasPrefix(bytes.TrimSpace(u.Path), []byte(`"`)) {
					path = strings.TrimPrefix(path, "/")
				}
				text += "/" + path
			}
			if u.Hash != "" {
				text += "#" + u.Hash
			}
		}
		if present(u.Query) {
			structuredQuery = true
			if err := json.Unmarshal(u.Query, &query); err != nil {
				return fmt.Errorf("URL query must be an array: %w", err)
			}
		}
		vars = u.Variable
	}
	base, fragment, _ := cutOutsideReference(text, '#')
	base, rawQuery, hasQuery := cutOutsideReference(base, '?')
	if structuredQuery {
		for _, q := range query {
			if q.Value == nil {
				p.warn(where, "query parameters without '=' are normalized to empty values")
			}
		}
		req.Query = convertPairs(query, transform, true)
	} else if hasQuery {
		for _, part := range splitOutsideReference(rawQuery, '&') {
			if part == "" {
				continue
			}
			key, value, equals := cutOutsideReference(part, '=')
			if !equals {
				p.warn(where, "query parameters without '=' are normalized to empty values")
			}
			req.Query = append(req.Query, model.KeyValue{Name: decodeQuery(escapeEncodedDollars(transform(key))), Value: decodeQuery(escapeEncodedDollars(transform(value))), Enabled: true})
		}
	}
	req.URL = transform(base)
	if fragment != "" {
		req.URL += "#" + transform(fragment)
	}
	for _, v := range vars {
		if v.Disabled {
			continue
		}
		name := v.name()
		// Posting matches whole path segments, whereas Postman supports :id.json.
		// Give the suffixed segment an equivalent independent path parameter.
		for _, segment := range model.PathParamNames(req.URL) {
			root, suffix, hasSuffix := strings.Cut(segment, ".")
			if root != name {
				continue
			}
			value := scalar(v.Value)
			if hasSuffix && value != "" {
				value += "." + suffix
			}
			value = transform(value)
			if decoded, err := url.PathUnescape(escapeEncodedDollars(value)); err == nil {
				value = decoded
			}
			req.PathParams = append(req.PathParams, model.KeyValue{Name: segment, Value: value, Enabled: true})
		}
	}
	return nil
}
func joinURLParts(raw json.RawMessage, sep string) (string, error) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if !present(part) {
			return "", fmt.Errorf("URL parts cannot be null")
		}
		if err := json.Unmarshal(part, &s); err != nil {
			var p struct{ Value *string }
			if sep != "/" || json.Unmarshal(part, &p) != nil || p.Value == nil {
				return "", fmt.Errorf("expected strings or path objects with a value")
			}
			s = *p.Value
		}
		values = append(values, s)
	}
	return strings.Join(values, sep), nil
}

func (p *parser) readAuth(req *model.Request, raw json.RawMessage, transform func(string) string, where string) error {
	if !present(raw) {
		return nil
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("invalid auth: %w", err)
	}
	var kind string
	if err := json.Unmarshal(data["type"], &kind); err != nil {
		return fmt.Errorf("auth type must be a string")
	}
	if kind == "noauth" {
		return nil
	}
	attrs := map[string]string{}
	if present(data[kind]) {
		var list []variable
		if err := json.Unmarshal(data[kind], &list); err == nil {
			for _, v := range list {
				attrs[v.name()] = scalar(v.Value)
			}
		} else {
			var old map[string]json.RawMessage
			if err := json.Unmarshal(data[kind], &old); err != nil {
				return fmt.Errorf("invalid %s auth attributes", kind)
			}
			for k, v := range old {
				attrs[k] = scalar(v)
			}
		}
	}
	switch kind {
	case "basic", "digest":
		req.Auth = model.Auth{Type: model.AuthType(kind), Username: transform(attrs["username"]), Password: transform(attrs["password"])}
		if kind == "digest" && len(attrs) > 2 {
			p.warn(where, "Digest challenge overrides are not imported; Posting negotiates with the server")
		}
	case "bearer":
		req.Auth = model.Auth{Type: model.AuthBearer, Token: transform(attrs["token"])}
	case "apikey":
		kv := model.KeyValue{Name: transform(attrs["key"]), Value: transform(attrs["value"]), Enabled: true}
		if kv.Name == "" && kv.Value == "" {
			return nil
		}
		if len(model.FindVariables(kv.Name)) > 0 {
			p.warn(where, "API key name uses variables; existing-field override is based on imported defaults and should be reviewed if the name changes")
		}
		// Postman's authorizer removes every matching field before adding the
		// helper value. Headers match without case, query keys exactly.
		queryTarget := model.Substitute(transform(attrs["in"]), model.MapLookup(p.defaults)) == "query"
		if queryTarget {
			kv.Name = decodeQuery(escapeEncodedDollars(kv.Name))
			kv.Value = decodeQuery(escapeEncodedDollars(kv.Value))
		}
		defaults := map[string]string{}
		for _, v := range p.result.Variables {
			defaults[v.Name] = v.Value
		}
		resolvedKey := model.Substitute(kv.Name, model.MapLookup(defaults))
		remove := func(fields []model.KeyValue, header bool) []model.KeyValue {
			kept := fields[:0]
			for _, field := range fields {
				name := model.Substitute(field.Name, model.MapLookup(defaults))
				same := name == resolvedKey
				if header {
					same = strings.EqualFold(name, resolvedKey)
				}
				if !same {
					kept = append(kept, field)
				}
			}
			return kept
		}
		if queryTarget {
			req.Query = append(remove(req.Query, false), kv)
		} else {
			req.Headers = append(remove(req.Headers, true), kv)
		}
	default:
		p.warn(where, "auth type %q is unsupported; configure authentication manually", kind)
	}
	return nil
}

// template escapes literal dollars before translating Postman's double braces.
// Local overrides are expanded in isolation; root defaults remain editable.
func (p *parser) template(s string, local map[string]string, where string) string {
	s = p.expand(s, local, map[string]bool{}, where)
	s = strings.ReplaceAll(s, "$", "$$")
	return reference.ReplaceAllStringFunc(s, func(ref string) string {
		name := ref[2 : len(ref)-2]
		if identifier.MatchString(name) {
			return "${" + name + "}"
		}
		p.warn(where, "variable reference %q is unsupported (dynamic, vault, or non-identifier name); left literal", strings.ReplaceAll(ref, "$$", "$"))
		return ref
	})
}

// expand is deliberately bounded: cycles or exponentially growing templates in
// untrusted collections must not hang an import or allocate unbounded memory.
func (p *parser) expand(s string, values map[string]string, stack map[string]bool, where string) string {
	budget := 1024
	return p.expandBudget(s, values, stack, where, &budget)
}

func (p *parser) expandBudget(s string, values map[string]string, stack map[string]bool, where string, budget *int) string {
	*budget--
	if *budget < 0 {
		p.warn(where, "variable expansion exceeds work limit; unresolved references retained")
		return s
	}
	if p.err != nil {
		return s
	}
	const limit = 1 << 20
	if len(stack) > 64 {
		p.warn(where, "variable expansion exceeds 64 levels; unresolved references remain literal")
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range reference.FindAllStringIndex(s, -1) {
		b.WriteString(s[last:loc[0]])
		ref := s[loc[0]:loc[1]]
		name := ref[2 : len(ref)-2]
		value, ok := values[name]
		if ok && !stack[name] {
			stack[name] = true
			value = p.expandBudget(value, values, stack, where, budget)
			delete(stack, name)
		} else {
			value = ref
			if ok {
				p.warn(where, "cyclic variable %q remains unresolved", name)
			}
		}
		if b.Len()+len(value) > limit {
			p.warn(where, "variable expansion exceeds 1 MiB; original references retained")
			return s
		}
		if ok {
			p.expansionBytes += len(value)
			if p.expansionBytes > 32<<20 {
				p.err = fmt.Errorf("postman: total variable expansion exceeds 32 MiB")
				return s
			}
		}
		b.WriteString(value)
		last = loc[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// A collection alias may transitively refer to a locally overridden variable.
// Materialize only those aliases, retaining independent collection defaults as
// editable Posting variables. Sort keys so work limits remain deterministic.
func (p *parser) scopedAliases(local map[string]string) map[string]string {
	if len(local) == 0 {
		return local
	}
	out := make(map[string]string, len(local))
	for k, v := range local {
		out[k] = v
	}
	keys := make([]string, 0, len(p.defaults))
	for k := range p.defaults {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for pass := 0; pass < 64; pass++ {
		changed := false
		for _, key := range keys {
			if _, ok := out[key]; ok {
				continue
			}
			for _, ref := range reference.FindAllString(p.defaults[key], -1) {
				if _, ok := out[ref[2:len(ref)-2]]; ok {
					out[key] = p.defaults[key]
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	return out
}

// Delimiters inside a Postman variable name belong to the template, not the
// URL. Even unsupported variable names must remain intact for diagnostics.
func cutOutsideReference(s string, delimiter byte) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "{{") {
			if end := strings.Index(s[i+2:], "}}"); end >= 0 {
				i += end + 3
				continue
			}
		}
		if s[i] == delimiter {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
func splitOutsideReference(s string, delimiter byte) []string {
	var parts []string
	for {
		before, after, ok := cutOutsideReference(s, delimiter)
		parts = append(parts, before)
		if !ok {
			return parts
		}
		s = after
	}
}

// Decode only after interpreting source templates. Otherwise %7B%7BNAME%7D%7D
// accidentally becomes a variable. Newly decoded dollars must remain literal
// in Posting, while existing escaped dollars and generated ${NAME} references
// retain the template converter's meaning.
func escapeEncodedDollars(s string) string { return strings.ReplaceAll(s, "%24", "$$") }
