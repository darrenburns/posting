package curl

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

// FormatOptions control how Format writes a command.
type FormatOptions struct {
	// ExtraArgs are inserted after "curl" as written (the
	// curl_export_extra_args setting).
	ExtraArgs string
	// Multiline puts each option on its own line.
	Multiline bool
}

// Format writes req as a curl command. Pass a resolved request (see
// model.Resolve) for a command that runs as-is; an unresolved one keeps its
// $VARIABLE and ${VARIABLE} references intact for later substitution.
//
// Only what goes on the wire is written: the request's name, description,
// disabled rows and Posting-only options have no curl equivalent.
func Format(req model.Request, opts FormatOptions) string {
	parts := []string{"curl"}
	if extra := strings.TrimSpace(opts.ExtraArgs); extra != "" {
		parts = append(parts, extra)
	}
	hasBody := req.Body.Type == model.BodyRaw && req.Body.Raw != "" || req.Body.Type == model.BodyForm && anyEnabled(req.Body.Form)
	switch {
	case req.Method == model.MethodHead && !hasBody:
		parts = append(parts, "--head")
	case req.Method == model.MethodPost && hasBody, req.Method == model.MethodGet && !hasBody, req.Method == "":
		// curl's default for the data given.
	default:
		parts = append(parts, "-X", string(req.Method))
	}
	parts = append(parts, Quote(targetURL(req)))

	var headers []model.KeyValue
	for _, h := range req.Headers {
		if h.Enabled && h.Name != "" {
			headers = append(headers, h)
		}
	}
	// A raw body needs its type spelled out unless the headers already do,
	// or curl (and Parse) would take it for a form.
	if req.Body.Type == model.BodyRaw && hasBody && headerValue(headers, "Content-Type") == "" {
		headers = append(headers, model.KeyValue{Name: "Content-Type", Value: orDefault(req.Body.ContentType, "text/plain"), Enabled: true})
	}
	for _, h := range headers {
		parts = append(parts, "-H", Quote(h.Name+": "+h.Value))
	}

	switch req.Auth.Type {
	case model.AuthBasic:
		parts = append(parts, "-u", Quote(req.Auth.Username+":"+req.Auth.Password))
	case model.AuthDigest:
		parts = append(parts, "--digest", "-u", Quote(req.Auth.Username+":"+req.Auth.Password))
	case model.AuthBearer:
		parts = append(parts, "-H", Quote("Authorization: Bearer "+req.Auth.Token))
	}

	switch req.Body.Type {
	case model.BodyRaw:
		if req.Body.Raw != "" {
			parts = append(parts, "--data-raw", Quote(req.Body.Raw))
		}
	case model.BodyForm:
		for _, f := range req.Body.Form {
			if f.Enabled && f.Name != "" {
				parts = append(parts, "--data-urlencode", Quote(f.Name+"="+f.Value))
			}
		}
	}

	if req.Options.FollowRedirects {
		parts = append(parts, "-L")
	} else {
		parts = append(parts, "--no-location")
	}
	if !req.Options.VerifySSL {
		parts = append(parts, "-k")
	}
	if t := req.Options.TimeoutSeconds; t > 0 && t != model.DefaultOptions().TimeoutSeconds {
		parts = append(parts, "-m", strconv.FormatFloat(t, 'f', -1, 64))
	}
	if req.Options.ProxyURL != "" {
		parts = append(parts, "-x", Quote(req.Options.ProxyURL))
	}

	if !opts.Multiline {
		return strings.Join(parts, " ")
	}
	// Keep each option with its value on one line.
	var lines []string
	line := parts[0]
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "-") && line != "curl" && !strings.HasSuffix(line, " -X") {
			lines = append(lines, line)
			line = p
			continue
		}
		line += " " + p
	}
	lines = append(lines, line)
	return strings.Join(lines, " \\\n  ")
}

// targetURL is the URL with the enabled query parameters, for requests
// whose query lives only in their parameter list.
func targetURL(req model.Request) string {
	base, fragment, hasFragment := strings.Cut(req.URL, "#")
	if strings.Contains(base, "?") {
		return req.URL
	}
	var pairs []string
	for _, kv := range req.Query {
		if kv.Enabled {
			pairs = append(pairs, escapeQueryPart(kv.Name)+"="+escapeQueryPart(kv.Value))
		}
	}
	if len(pairs) == 0 {
		return req.URL
	}
	target := base + "?" + strings.Join(pairs, "&")
	if hasFragment {
		target += "#" + fragment
	}
	return target
}

func anyEnabled(kvs []model.KeyValue) bool {
	for _, kv := range kvs {
		if kv.Enabled && kv.Name != "" {
			return true
		}
	}
	return false
}

// Keep unresolved Posting references intact while escaping literal query text.
func escapeQueryPart(value string) string {
	var out strings.Builder
	end := 0
	for _, ref := range model.FindVariables(value) {
		out.WriteString(url.QueryEscape(value[end:ref.Start]))
		out.WriteString(value[ref.Start:ref.End])
		end = ref.End
	}
	out.WriteString(url.QueryEscape(value[end:]))
	return out.String()
}
