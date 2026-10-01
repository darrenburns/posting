package bruno

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

type scope struct {
	headers, query []model.KeyValue
	vars           map[string]string
	auth           model.Auth
	authMode       string
	authFields     map[string]string
}

func emptyScope() scope {
	return scope{vars: map[string]string{}, auth: model.Auth{Type: model.AuthNone}}
}

// Parse imports one standalone .bru request. Parent collection/folder settings
// are available only through Load of the collection directory.
func Parse(data []byte) (importing.Result, error) {
	d, err := parseDocument(data)
	if err != nil {
		return importing.Result{}, err
	}
	result := importing.Result{}
	r, err := convert(d, emptyScope(), &result)
	if err != nil {
		return importing.Result{}, err
	}
	if r != nil {
		result.Name = r.Name
		result.Requests = append(result.Requests, *r)
	}
	return result, nil
}
func warn(result *importing.Result, s string) {
	if len(result.Warnings) >= 500 {
		if len(result.Warnings) == 500 {
			result.Warnings = append(result.Warnings, "additional Bruno warnings omitted")
		}
		return
	}
	for _, w := range result.Warnings {
		if w == s {
			return
		}
	}
	result.Warnings = append(result.Warnings, s)
}
func diagnostics(d document, result *importing.Result) {
	for _, b := range d {
		if b.name == "body:multipart-form" || b.name == "body:file" {
			warn(result, b.name+" is not imported")
		}
		if strings.HasPrefix(b.name, "script:") || b.name == "tests" || b.name == "assert" || b.name == "vars:post-response" || b.name == "example" || b.name == "app" {
			warn(result, b.name+" is not imported or executed")
		}
		if !isText(b.name) && strings.Contains(b.text, "@") {
			for _, line := range strings.Split(b.text, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "@") && !strings.Contains(line, ":") {
					warn(result, "Bruno variable type/description annotations are not preserved")
				}
			}
		}
	}
}
func applyScope(d document, parent scope, result *importing.Result) (scope, error) {
	s := parent
	s.vars = map[string]string{}
	for k, v := range parent.vars {
		s.vars[k] = v
	}
	for _, name := range []string{"headers", "query", "vars:pre-request"} {
		rows, err := d.pairs(name)
		if err != nil {
			return s, err
		}
		switch name {
		case "headers":
			s.headers = mergeRows(parent.headers, rows, true)
		case "query":
			s.query = mergeQuery(parent.query, rows)
		default:
			for _, v := range rows {
				if v.Enabled {
					s.vars[strings.TrimPrefix(v.Name, "@")] = v.Value
				}
			}
		}
	}
	if d.has("auth") {
		p, err := d.pairs("auth")
		if err != nil {
			return s, err
		}
		mode := values(p)["mode"]
		if mode != "" && mode != "inherit" {
			s.authMode = mode
			p, err = d.pairs("auth:" + mode)
			if err != nil {
				return s, err
			}
			s.authFields = values(p)
		}
	}
	diagnostics(d, result)
	for _, b := range d {
		switch {
		case b.name == "meta", b.name == "headers", b.name == "query", b.name == "auth", strings.HasPrefix(b.name, "auth:"), b.name == "vars:pre-request", b.name == "docs", b.name == "vars:post-response", b.name == "tests", strings.HasPrefix(b.name, "script:"):
		default:
			warn(result, "unsupported collection/folder block "+b.name)
		}
	}
	return s, nil
}

// Disabled child rows do not turn off enabled inherited rows in Bruno.
func mergeRows(parent, child []model.KeyValue, fold bool) []model.KeyValue {
	out := []model.KeyValue{}
	positions := map[string]int{}
	for _, rows := range [][]model.KeyValue{parent, child} {
		for _, row := range rows {
			key := row.Name
			if fold {
				key = strings.ToLower(key)
			}
			if i, ok := positions[key]; ok && row.Enabled {
				out[i] = row
				continue
			}
			if row.Enabled {
				positions[key] = len(out)
			}
			out = append(out, row)
		}
	}
	return out
}

func convert(d document, parent scope, result *importing.Result) (*model.Request, error) {
	meta, err := d.pairs("meta")
	if err != nil {
		return nil, err
	}
	m := values(meta)
	if t := m["type"]; t != "" && t != "http" && t != "graphql" {
		warn(result, "skipped unsupported Bruno request type "+t)
		return nil, nil
	}
	var method string
	for _, b := range d {
		switch b.name {
		case "get", "post", "put", "delete", "patch", "head", "options", "connect", "trace", "http":
			if method != "" {
				return nil, fmt.Errorf("multiple HTTP method blocks")
			}
			method = b.name
		}
	}
	if method == "" {
		return nil, fmt.Errorf("missing HTTP method block")
	}
	hp, err := d.pairs(method)
	if err != nil {
		return nil, err
	}
	http := values(hp)
	actualMethod := strings.ToUpper(method)
	if method == "http" {
		actualMethod = strings.ToUpper(http["method"])
	}
	valid := false
	for _, candidate := range model.Methods {
		if string(candidate) == actualMethod {
			valid = true
		}
	}
	if !valid {
		warn(result, "skipped HTTP method "+actualMethod+" unsupported by Posting request files")
		return nil, nil
	}
	if http["url"] == "" {
		return nil, fmt.Errorf("HTTP request URL is empty")
	}
	r := model.NewRequest()
	r.Name = m["name"]
	if r.Name == "" {
		r.Name = "Imported request"
	}
	r.Method = model.Method(actualMethod)
	r.URL = http["url"]
	r.Description = d.text("docs")
	r.File = r.Name + ".posting.yaml"
	vars := map[string]string{}
	for k, v := range parent.vars {
		vars[k] = v
	}
	vp, err := d.pairs("vars:pre-request")
	if err != nil {
		return nil, err
	}
	for _, v := range vp {
		if v.Enabled {
			vars[strings.TrimPrefix(v.Name, "@")] = v.Value
		}
	}
	expansion := expander{vars: vars, result: result}
	headers, err := d.pairs("headers")
	if err != nil {
		return nil, err
	}
	r.Headers = mergeRows(parent.headers, headers, true)
	query, err := d.pairs("params:query")
	if err != nil {
		return nil, err
	}
	legacy, err := d.pairs("query")
	if err != nil {
		return nil, err
	}
	query = append(legacy, query...)
	// The URL and params:query are two serialized views of the same rows. Keep
	// explicit rows (including disabled rows); only add URL keys absent there.
	rawURL := r.URL
	fragment := ""
	if i := strings.IndexByte(rawURL, '#'); i >= 0 {
		fragment = rawURL[i:]
		rawURL = rawURL[:i]
	}
	explicitQuery := map[string]bool{}
	for _, q := range query {
		explicitQuery[q.Name] = true
	}
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		for _, part := range strings.Split(rawURL[i+1:], "&") {
			if part == "" {
				continue
			}
			parts := strings.SplitN(part, "=", 2)
			key, e := url.QueryUnescape(parts[0])
			if e != nil {
				return nil, fmt.Errorf("invalid URL query: %w", e)
			}
			value := ""
			if len(parts) == 2 {
				value, e = url.QueryUnescape(parts[1])
				if e != nil {
					return nil, fmt.Errorf("invalid URL query: %w", e)
				}
			}
			found := explicitQuery[key]
			if !found {
				r.Query = append(r.Query, model.KeyValue{Name: key, Value: value, Enabled: true})
			}
		}
		rawURL = rawURL[:i]
	}
	r.URL = rawURL + fragment
	r.Query = mergeQuery(parent.query, append(r.Query, query...))
	r.PathParams, err = d.pairs("params:path")
	if err != nil {
		return nil, err
	}
	mode := http["auth"]
	fields := map[string]string{}
	if mode == "inherit" {
		mode = parent.authMode
		fields = parent.authFields
		if mode == "" {
			warn(result, "auth: inherit has no available parent authentication")
		}
	} else {
		p, e := d.pairs("auth:" + mode)
		if e != nil {
			return nil, e
		}
		fields = values(p)
	}
	switch mode {
	case "", "none":
	case "basic", "digest":
		r.Auth = model.Auth{Type: model.AuthType(mode), Username: fields["username"], Password: fields["password"]}
	case "bearer":
		r.Auth = model.Auth{Type: model.AuthBearer, Token: fields["token"]}
	case "apikey":
		kv := model.KeyValue{Name: fields["key"], Value: fields["value"], Enabled: true}
		switch fields["placement"] {
		case "header":
			r.Headers = mergeRows(r.Headers, []model.KeyValue{kv}, true)
		case "queryparams":
			r.Query = mergeQuery(r.Query, []model.KeyValue{kv})
		default:
			warn(result, "unsupported API key placement "+fields["placement"])
		}
	default:
		warn(result, "unsupported authentication "+mode+"; credentials were not imported")
	}
	bodyMode := http["body"]
	switch bodyMode {
	case "", "none":
	case "json", "text", "xml", "sparql":
		name := "body:" + bodyMode
		if !d.has(name) {
			return nil, fmt.Errorf("missing %s block", name)
		}
		ct := map[string]string{"json": "application/json", "text": "text/plain", "xml": "application/xml", "sparql": "application/sparql-query"}[bodyMode]
		r.Body = model.Body{Type: model.BodyRaw, Raw: d.text(name), ContentType: ct}
	case "form-urlencoded":
		r.Body.Type = model.BodyForm
		r.Body.Form, err = d.pairs("body:form-urlencoded")
		if err != nil {
			return nil, err
		}
	case "graphql":
		// Variables in both are expanded below, with the other fields.
		r.Payload = model.GraphQL{Query: d.text("body:graphql"), Variables: d.text("body:graphql:vars")}
	default:
		warn(result, "unsupported body mode "+bodyMode+"; body was not imported")
	}
	if d.has("body") {
		warn(result, "legacy body block is not imported")
	}
	settings, err := d.pairs("settings")
	if err != nil {
		return nil, err
	}
	for _, v := range settings {
		switch v.Name {
		case "followRedirects":
			if v.Value != "true" && v.Value != "false" {
				return nil, fmt.Errorf("invalid followRedirects")
			}
			r.Options.FollowRedirects = v.Value == "true"
		case "timeout":
			if v.Value == "inherit" {
				warn(result, "inherited timeout uses Posting default")
				continue
			}
			n, e := strconv.ParseFloat(v.Value, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 86400000 {
				return nil, fmt.Errorf("invalid timeout")
			}
			r.Options.TimeoutSeconds = n / 1000
		default:
			warn(result, "unsupported request setting "+v.Name)
		}
	}
	diagnostics(d, result)
	for _, b := range d {
		switch {
		case b.name == "meta", b.name == method, b.name == "headers", b.name == "query", b.name == "params:query", b.name == "params:path", b.name == "vars:pre-request", b.name == "vars:post-response", b.name == "settings", b.name == "docs", b.name == "body", b.name == "tests", b.name == "assert", b.name == "example", b.name == "app", strings.HasPrefix(b.name, "body:"), strings.HasPrefix(b.name, "auth:"), strings.HasPrefix(b.name, "script:"):
		default:
			warn(result, "unsupported block "+b.name)
		}
	}
	r.URL = expansion.expand(r.URL, nil)
	r.Auth.Username = expansion.expand(r.Auth.Username, nil)
	r.Auth.Password = expansion.expand(r.Auth.Password, nil)
	r.Auth.Token = expansion.expand(r.Auth.Token, nil)
	for _, rows := range [][]model.KeyValue{r.Headers, r.Query, r.PathParams, r.Body.Form} {
		for i := range rows {
			rows[i].Name = expansion.expand(rows[i].Name, nil)
			rows[i].Value = expansion.expand(rows[i].Value, nil)
		}
	}
	// Bruno selects JSON string escaping from the effective Content-Type after
	// header interpolation, rather than from the body editor's selected mode.
	contentType := r.Body.ContentType
	for _, header := range r.Headers {
		if header.Enabled && strings.EqualFold(header.Name, "Content-Type") {
			contentType = header.Value
		}
	}
	if strings.Contains(contentType, "${") || strings.Contains(contentType, "{{") {
		warn(result, "unresolved Content-Type uses import-time body escaping; check body variables after setting the header")
	}
	r.Body.Raw = expansion.expandMode(r.Body.Raw, nil, strings.Contains(contentType, "json"))
	if g, ok := r.Payload.(model.GraphQL); ok {
		r.Payload = model.GraphQL{
			Query:     importing.BracedOnly(expansion.expand(g.Query, nil)),
			Variables: expansion.expandMode(g.Variables, nil, true),
		}
	}
	if expansion.err != nil {
		return nil, expansion.err
	}
	r = model.Normalize(r)
	return &r, nil
}

var reference = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type expander struct {
	vars   map[string]string
	result *importing.Result
	count  int
	bytes  int
	err    error
}

func (e *expander) expand(s string, stack []string) string { return e.expandMode(s, stack, false) }
func (e *expander) expandMode(s string, stack []string, jsonBody bool) string {
	if e.err != nil {
		return ""
	}
	// Escape native Posting references in literal Bruno input. Only Bruno's
	// {{...}} references should acquire Posting substitution behavior.
	s = strings.ReplaceAll(s, "$", "$$")
	var out strings.Builder
	last := 0
	inString, escaped := false, false
	for _, loc := range reference.FindAllStringIndex(s, -1) {
		if jsonBody {
			for _, c := range s[last:loc[0]] {
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' && inString {
					escaped = true
					continue
				}
				if c == '"' {
					inString = !inString
				}
			}
		}
		ref := s[loc[0]:loc[1]]
		replacement := func() string {
			name := strings.TrimSpace(reference.FindStringSubmatch(ref)[1])
			e.count++
			if len(stack) >= 32 || e.count > 10000 {
				e.err = fmt.Errorf("Bruno variable expansion limit reached")
				return ref
			}
			for _, key := range stack {
				if key == name {
					warn(e.result, "cyclic variable reference "+name+" retained")
					return ref
				}
			}
			if value, ok := e.vars[name]; ok {
				return e.expand(value, append(stack, name))
			}
			if strings.HasPrefix(name, "process.env.") {
				name = strings.TrimPrefix(name, "process.env.")
			}
			if identifier.MatchString(name) {
				return "${" + name + "}"
			}
			warn(e.result, "unsupported variable reference "+name+" retained")
			return ref
		}()
		if jsonBody && inString {
			if strings.Contains(replacement, "${") || strings.Contains(replacement, "{{") {
				warn(e.result, "unresolved JSON body variables require JSON-escaped values in Posting")
			}
			quoted, _ := json.Marshal(replacement)
			replacement = string(quoted[1 : len(quoted)-1])
		}
		if out.Len()+loc[0]-last+len(replacement) > maxFileSize {
			e.err = fmt.Errorf("expanded Bruno value exceeds 16 MiB")
			return ""
		}
		out.WriteString(s[last:loc[0]])
		out.WriteString(replacement)
		last = loc[1]
	}
	if out.Len()+len(s)-last > maxFileSize {
		e.err = fmt.Errorf("expanded Bruno value exceeds 16 MiB")
		return ""
	}
	out.WriteString(s[last:])
	// Bound total allocation work across fields and recursive substitutions,
	// not just each field. A large static value reused by thousands of small
	// references otherwise multiplies a bounded input into gigabytes.
	e.bytes += out.Len()
	if e.bytes > maxFileSize {
		e.err = fmt.Errorf("Bruno aggregate variable expansion exceeds 16 MiB")
		return ""
	}
	return out.String()
}

func mergeQuery(parent, child []model.KeyValue) []model.KeyValue {
	names := map[string]bool{}
	for _, c := range child {
		if c.Enabled {
			names[c.Name] = true
		}
	}
	var result []model.KeyValue
	for _, p := range parent {
		if !names[p.Name] {
			result = append(result, p)
		}
	}
	return append(result, child...)
}
