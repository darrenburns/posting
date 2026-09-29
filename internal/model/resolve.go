package model

import (
	"net/url"
	"regexp"
	"strings"
)

// UndefinedVariablesError reports variables in the URL that have no value.
// A URL with a variable left in it can't be sent, so resolving fails rather
// than sending "${BASE_URL}/users" literally.
type UndefinedVariablesError struct {
	Names []string
}

func (e UndefinedVariablesError) Error() string {
	refs := make([]string, len(e.Names))
	for i, name := range e.Names {
		refs[i] = "$" + name
	}
	if len(refs) == 1 {
		return "Variable not defined: " + refs[0]
	}
	return "Variables not defined: " + strings.Join(refs, ", ")
}

// Resolve returns req as it goes on the wire: variables substituted, path
// parameters filled in, the enabled query parameters merged into the URL and
// a scheme added if the URL has none. Query is empty in the result, since
// the URL carries it.
//
// The body is only substituted when the request's SubstituteBodyVariables
// option is on; otherwise it is sent exactly as written.
func Resolve(req Request, lookup func(string) (string, bool)) (Request, error) {
	out := req.Clone()
	sub := func(s string) string { return Substitute(s, lookup) }

	for i := range out.PathParams {
		out.PathParams[i].Value = sub(out.PathParams[i].Value)
	}
	base, query, fragment := splitURL(req.URL)
	if missing := undefined(base, lookup); len(missing) > 0 {
		return Request{}, UndefinedVariablesError{Names: missing}
	}
	base = ensureScheme(ResolvePathParams(sub(base), out.PathParams))

	var pairs []string
	if query != "" {
		for _, part := range strings.Split(query, "&") {
			if part == "" {
				continue
			}
			name, value, hasValue := strings.Cut(part, "=")
			name, value = unescapeQuery(name), unescapeQuery(value)
			pair := url.QueryEscape(sub(name))
			if hasValue {
				pair += "=" + url.QueryEscape(sub(value))
			}
			pairs = append(pairs, pair)
		}
	} else {
		// Requests built outside the editor (such as a Posting 2 file that was
		// never opened) may keep their query only in the parameter list.
		for _, kv := range req.Query {
			if kv.Enabled {
				pairs = append(pairs, url.QueryEscape(sub(kv.Name))+"="+url.QueryEscape(sub(kv.Value)))
			}
		}
	}
	out.URL = base
	if len(pairs) > 0 {
		out.URL += "?" + strings.Join(pairs, "&")
	}
	out.URL += fragment
	out.Query = nil

	for i := range out.Headers {
		out.Headers[i].Name = sub(out.Headers[i].Name)
		out.Headers[i].Value = sub(out.Headers[i].Value)
	}
	out.Auth.Username = sub(out.Auth.Username)
	out.Auth.Password = sub(out.Auth.Password)
	out.Auth.Token = sub(out.Auth.Token)
	out.Options.ProxyURL = sub(out.Options.ProxyURL)
	if req.Options.SubstituteBodyVariables {
		out.Body.Raw = sub(out.Body.Raw)
		for i := range out.Body.Form {
			out.Body.Form[i].Name = sub(out.Body.Form[i].Name)
			out.Body.Form[i].Value = sub(out.Body.Form[i].Value)
		}
	}
	return out, nil
}

// splitURL separates the query string and fragment from the rest of a URL.
// The query is returned without its "?".
func splitURL(raw string) (base, query, fragment string) {
	base = raw
	if i := strings.IndexByte(base, '#'); i >= 0 {
		base, fragment = base[:i], base[i:]
	}
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base, query = base[:i], base[i+1:]
	}
	return base, query, fragment
}

func unescapeQuery(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

// undefined lists the variables referenced in s that lookup can't resolve.
func undefined(s string, lookup func(string) (string, bool)) []string {
	var names []string
	seen := map[string]bool{}
	for _, ref := range FindVariables(s) {
		if _, ok := lookup(ref.Name); !ok && !seen[ref.Name] {
			seen[ref.Name] = true
			names = append(names, ref.Name)
		}
	}
	return names
}

var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// ensureScheme defaults URLs without a scheme to http://, as Posting 2 does.
func ensureScheme(u string) string {
	if u == "" || schemePattern.MatchString(u) {
		return u
	}
	return "http://" + u
}

// MapLookup adapts a map to the lookup function Substitute and Resolve take.
func MapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	}
}
