// Package curl converts between requests and curl commands.
//
// Parse understands the commands browsers produce with "Copy as cURL" and
// the ones people write by hand; Format writes commands that Parse reads
// back to the same request, so exporting and importing is lossless for
// everything curl can express.
package curl

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/darrenburns/posting/v3/internal/model"
)

// IsCommand reports whether text looks like a curl command.
func IsCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed == "curl" || strings.HasPrefix(trimmed, "curl ") || strings.HasPrefix(trimmed, "curl\t") || strings.HasPrefix(trimmed, "curl\\")
}

// flagsWithValues are curl options that take an argument. Options Posting
// doesn't use are still listed so their argument isn't mistaken for the URL.
var flagsWithValues = map[string]bool{
	"-X": true, "--request": true, "-H": true, "--header": true,
	"-d": true, "--data": true, "--data-raw": true, "--data-binary": true, "--data-ascii": true, "--data-urlencode": true, "--json": true,
	"-F": true, "--form": true, "--form-string": true,
	"-u": true, "--user": true, "-b": true, "--cookie": true, "-A": true, "--user-agent": true, "-e": true, "--referer": true,
	"-m": true, "--max-time": true, "-x": true, "--proxy": true, "--url": true,
	"-o": true, "--output": true, "-w": true, "--write-out": true, "-c": true, "--cookie-jar": true,
	"--connect-timeout": true, "--retry": true, "--retry-delay": true, "--retry-max-time": true, "--max-redirs": true,
	"--cacert": true, "--capath": true, "-E": true, "--cert": true, "--key": true, "--cert-type": true, "--key-type": true, "--pass": true,
	"-r": true, "--range": true, "-T": true, "--upload-file": true, "--resolve": true, "--connect-to": true, "-U": true, "--proxy-user": true,
	"-K": true, "--config": true, "--limit-rate": true, "-y": true, "--speed-time": true, "-Y": true, "--speed-limit": true,
	"--interface": true, "--dns-servers": true, "--oauth2-bearer": true, "--aws-sigv4": true, "-D": true, "--dump-header": true,
	"--trace": true, "--trace-ascii": true, "--stderr": true, "-z": true, "--time-cond": true, "--variable": true, "--expand-data": true,
	"-Q": true, "--quote": true, "-t": true, "--telnet-option": true, "--unix-socket": true, "--abstract-unix-socket": true,
}

// Parse builds a request from a curl command.
func Parse(command string) (model.Request, error) {
	tokens, err := Split(command)
	if err != nil {
		return model.Request{}, err
	}
	if len(tokens) == 0 || tokens[0] != "curl" {
		return model.Request{}, fmt.Errorf("not a curl command")
	}
	req := model.NewRequest()
	var (
		method     string
		rawURL     string
		data       []string
		dataIsJSON bool
		form       []model.KeyValue
		user       string
		digest     bool
		getMode    bool
		headMode   bool
		followSet  bool
		follow     bool
		cookies    []string
	)
	addHeader := func(raw string) {
		name, value, ok := strings.Cut(raw, ":")
		if !ok {
			// "Name;" sends an empty header in curl.
			if strings.HasSuffix(raw, ";") {
				req.Headers = append(req.Headers, model.KeyValue{Name: strings.TrimSuffix(raw, ";"), Enabled: true})
			}
			return
		}
		req.Headers = append(req.Headers, model.KeyValue{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value), Enabled: true})
	}

	args := tokens[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var flag, value string
		hasValue := false
		switch {
		case arg == "--":
			if i+1 < len(args) && rawURL == "" {
				rawURL = args[i+1]
			}
			i = len(args)
			continue
		case strings.HasPrefix(arg, "--"):
			flag = arg
			if name, v, ok := strings.Cut(arg, "="); ok && flagsWithValues[name] {
				flag, value, hasValue = name, v, true
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Short options may be clustered (-sSL) and may carry their
			// value in the same word (-XPOST).
			cluster := arg[1:]
			for j := 0; j < len(cluster); j++ {
				short := "-" + string(cluster[j])
				if flagsWithValues[short] {
					flag = short
					if j+1 < len(cluster) {
						value, hasValue = cluster[j+1:], true
					}
					break
				}
				applyBoolean(short, &req, &followSet, &follow, &digest, &getMode, &headMode)
			}
			if flag == "" {
				continue
			}
		default:
			if rawURL == "" {
				rawURL = arg
			}
			continue
		}
		if flagsWithValues[flag] && !hasValue {
			if i+1 >= len(args) {
				return model.Request{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			value = args[i]
		}
		switch flag {
		case "-X", "--request":
			method = strings.ToUpper(value)
		case "-H", "--header":
			addHeader(value)
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii":
			data = append(data, value)
		case "--data-urlencode":
			data = append(data, urlencodeData(value))
		case "--json":
			data = append(data, value)
			dataIsJSON = true
		case "-F", "--form", "--form-string":
			name, v, _ := strings.Cut(value, "=")
			form = append(form, model.KeyValue{Name: name, Value: v, Enabled: true})
		case "-u", "--user":
			user = value
		case "-b", "--cookie":
			cookies = append(cookies, value)
		case "-A", "--user-agent":
			req.Headers = append(req.Headers, model.KeyValue{Name: "User-Agent", Value: value, Enabled: true})
		case "-e", "--referer":
			req.Headers = append(req.Headers, model.KeyValue{Name: "Referer", Value: value, Enabled: true})
		case "-m", "--max-time":
			if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
				req.Options.TimeoutSeconds = seconds
			}
		case "-x", "--proxy":
			req.Options.ProxyURL = value
		case "--url":
			rawURL = value
		case "--oauth2-bearer":
			req.Auth = model.Auth{Type: model.AuthBearer, Token: value}
		default:
			applyBoolean(flag, &req, &followSet, &follow, &digest, &getMode, &headMode)
		}
	}
	if rawURL == "" {
		return model.Request{}, fmt.Errorf("the curl command has no URL")
	}
	if followSet {
		req.Options.FollowRedirects = follow
	}
	if len(cookies) > 0 {
		req.Headers = append(req.Headers, model.KeyValue{Name: "Cookie", Value: strings.Join(cookies, "; "), Enabled: true})
	}

	// Authorization headers become Posting auth, which is easier to edit.
	kept := req.Headers[:0]
	for _, h := range req.Headers {
		if strings.EqualFold(h.Name, "Authorization") && req.Auth.Type == model.AuthNone {
			scheme, credentials, _ := strings.Cut(h.Value, " ")
			switch strings.ToLower(scheme) {
			case "bearer":
				req.Auth = model.Auth{Type: model.AuthBearer, Token: strings.TrimSpace(credentials)}
				continue
			case "basic":
				if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(credentials)); err == nil {
					username, password, _ := strings.Cut(string(decoded), ":")
					req.Auth = model.Auth{Type: model.AuthBasic, Username: username, Password: password}
					continue
				}
			}
		}
		kept = append(kept, h)
	}
	req.Headers = kept
	if user != "" {
		username, password, _ := strings.Cut(user, ":")
		req.Auth = model.Auth{Type: model.AuthBasic, Username: username, Password: password}
		if digest {
			req.Auth.Type = model.AuthDigest
		}
	}

	joined := strings.Join(data, "&")
	switch {
	case getMode && len(data) > 0:
		// -G sends the data as the query string, before any fragment.
		base, fragment, hasFragment := strings.Cut(rawURL, "#")
		if strings.Contains(base, "?") {
			base += "&" + joined
		} else {
			base += "?" + joined
		}
		rawURL = base
		if hasFragment {
			rawURL += "#" + fragment
		}
	case len(form) > 0:
		req.Body = model.Body{Type: model.BodyForm, Form: form, ContentType: "application/x-www-form-urlencoded"}
	case len(data) > 0:
		contentType := headerValue(req.Headers, "Content-Type")
		if dataIsJSON {
			contentType = "application/json"
			if headerValue(req.Headers, "Content-Type") == "" {
				req.Headers = append(req.Headers, model.KeyValue{Name: "Content-Type", Value: contentType, Enabled: true})
			}
			if headerValue(req.Headers, "Accept") == "" {
				req.Headers = append(req.Headers, model.KeyValue{Name: "Accept", Value: "application/json", Enabled: true})
			}
		}
		mediaType, _, _ := strings.Cut(contentType, ";")
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
		switch {
		case mediaType == "application/x-www-form-urlencoded" || (mediaType == "" && looksLikeForm(joined)):
			req.Body = model.Body{Type: model.BodyForm, Form: parseForm(joined), ContentType: "application/x-www-form-urlencoded"}
		case mediaType == "" && json.Valid([]byte(joined)):
			req.Body = model.Body{Type: model.BodyRaw, Raw: joined, ContentType: "application/json"}
		default:
			req.Body = model.Body{Type: model.BodyRaw, Raw: joined, ContentType: orDefault(mediaType, "text/plain")}
		}
	}

	switch {
	case method != "":
		req.Method = model.Method(method)
	case headMode:
		req.Method = model.MethodHead
	case len(data) > 0 && !getMode, len(form) > 0:
		req.Method = model.MethodPost
	}
	if !validMethod(req.Method) {
		return model.Request{}, fmt.Errorf("unsupported method %q", req.Method)
	}

	req.URL = rawURL
	if _, query, ok := strings.Cut(rawURL, "?"); ok {
		query, _, _ = strings.Cut(query, "#")
		req.Query = parseForm(query)
	}
	return req, nil
}

func applyBoolean(flag string, req *model.Request, followSet, follow, digest, getMode, headMode *bool) {
	switch flag {
	case "-k", "--insecure":
		req.Options.VerifySSL = false
	case "-L", "--location", "--location-trusted":
		*followSet, *follow = true, true
	case "--no-location":
		*followSet, *follow = true, false
	case "--digest":
		*digest = true
	case "--basic":
		*digest = false
	case "-G", "--get":
		*getMode = true
	case "-I", "--head":
		*headMode = true
	}
}

// urlencodeData applies --data-urlencode's rules: "name=content" encodes
// the content, "content" and "=content" encode it all.
func urlencodeData(value string) string {
	if strings.HasPrefix(value, "=") {
		return url.QueryEscape(value[1:])
	}
	if name, content, ok := strings.Cut(value, "="); ok {
		return name + "=" + url.QueryEscape(content)
	}
	return url.QueryEscape(value)
}

// looksLikeForm reports whether data is name=value pairs, which curl -d
// sends as a form when no Content-Type is given.
func looksLikeForm(data string) bool {
	if data == "" || strings.ContainsAny(data, " \n{}[]\"") {
		return false
	}
	for _, pair := range strings.Split(data, "&") {
		if !strings.Contains(pair, "=") {
			return false
		}
	}
	return true
}

func parseForm(data string) []model.KeyValue {
	var out []model.KeyValue
	for _, pair := range strings.Split(data, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		out = append(out, model.KeyValue{Name: unescape(name), Value: unescape(value), Enabled: true})
	}
	return out
}

func unescape(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

func headerValue(headers []model.KeyValue, name string) string {
	for _, h := range headers {
		if h.Enabled && strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func validMethod(m model.Method) bool {
	for _, candidate := range model.Methods {
		if m == candidate {
			return true
		}
	}
	return false
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
