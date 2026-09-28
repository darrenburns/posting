package model

import (
	"net/url"
	"strings"
)

// Curl renders req as a curl command, resolving variables with lookup.
func Curl(req Request, lookup func(string) (string, bool)) string {
	sub := func(s string) string { return Substitute(s, lookup) }
	parts := []string{"curl"}
	if req.Method != MethodGet {
		parts = append(parts, "-X", string(req.Method))
	}
	target := sub(ResolvePathParams(req.URL, req.PathParams))
	parts = append(parts, shellQuote(target))
	for _, h := range req.Headers {
		if h.Enabled {
			parts = append(parts, "-H", shellQuote(h.Name+": "+sub(h.Value)))
		}
	}
	switch req.Auth.Type {
	case AuthBasic:
		parts = append(parts, "-u", shellQuote(sub(req.Auth.Username)+":"+sub(req.Auth.Password)))
	case AuthDigest:
		parts = append(parts, "--digest", "-u", shellQuote(sub(req.Auth.Username)+":"+sub(req.Auth.Password)))
	case AuthBearer:
		parts = append(parts, "-H", shellQuote("Authorization: Bearer "+sub(req.Auth.Token)))
	}
	body := func(s string) string {
		if req.Options.SubstituteBodyVariables {
			return sub(s)
		}
		return s
	}
	switch req.Body.Type {
	case BodyRaw:
		if req.Body.Raw != "" {
			parts = append(parts, "--data-raw", shellQuote(body(req.Body.Raw)))
		}
	case BodyForm:
		for _, f := range req.Body.Form {
			if f.Enabled {
				parts = append(parts, "--data-urlencode", shellQuote(f.Name+"="+body(f.Value)))
			}
		}
	}
	if req.Options.FollowRedirects {
		parts = append(parts, "-L")
	}
	if !req.Options.VerifySSL {
		parts = append(parts, "-k")
	}
	if req.Options.ProxyURL != "" {
		parts = append(parts, "-x", shellQuote(sub(req.Options.ProxyURL)))
	}
	return strings.Join(parts, " ")
}

// ResolvePathParams replaces ":name" path segments with their values.
func ResolvePathParams(rawURL string, params []KeyValue) string {
	if len(params) == 0 {
		return rawURL
	}
	values := map[string]string{}
	for _, p := range params {
		values[p.Name] = p.Value
	}
	prefix, rest := "", rawURL
	if i := strings.Index(rest, "://"); i >= 0 {
		j := strings.IndexByte(rest[i+3:], '/')
		if j < 0 {
			return rawURL
		}
		prefix, rest = rest[:i+3+j], rest[i+3+j:]
	}
	suffix := ""
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest, suffix = rest[:i], rest[i:]
	}
	segments := strings.Split(rest, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "::") {
			segments[i] = segment[1:]
			continue
		}
		if name, ok := strings.CutPrefix(segment, ":"); ok {
			if value, found := values[name]; found && value != "" {
				segments[i] = url.PathEscape(value)
			}
		}
	}
	return prefix + strings.Join(segments, "/") + suffix
}

func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!*?&;|<>(){}[]#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
