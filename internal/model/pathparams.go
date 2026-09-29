package model

import (
	"net/url"
	"strings"
)

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
