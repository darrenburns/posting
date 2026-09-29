package client

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"
	"strings"
)

// digestChallenge is a parsed WWW-Authenticate: Digest header (RFC 7616).
type digestChallenge struct {
	realm, nonce, opaque, algorithm, qop string
}

// parseDigestChallenge finds the first Digest challenge among the
// WWW-Authenticate headers.
func parseDigestChallenge(headers []string) (digestChallenge, bool) {
	for _, header := range headers {
		scheme, params, ok := strings.Cut(strings.TrimSpace(header), " ")
		if !ok || !strings.EqualFold(scheme, "Digest") {
			continue
		}
		values := parseAuthParams(params)
		c := digestChallenge{
			realm:     values["realm"],
			nonce:     values["nonce"],
			opaque:    values["opaque"],
			algorithm: values["algorithm"],
		}
		// Prefer qop=auth; auth-int isn't supported.
		for _, q := range strings.Split(values["qop"], ",") {
			if strings.TrimSpace(q) == "auth" {
				c.qop = "auth"
			}
		}
		if c.nonce != "" {
			return c, true
		}
	}
	return digestChallenge{}, false
}

// parseAuthParams splits `a="x, y", b=z` into a map, honouring quotes.
func parseAuthParams(s string) map[string]string {
	out := map[string]string{}
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(s[:eq]))
		s = s[eq+1:]
		var value string
		if strings.HasPrefix(s, `"`) {
			end := 1
			for end < len(s) && s[end] != '"' {
				if s[end] == '\\' {
					end++
				}
				end++
			}
			value = strings.ReplaceAll(s[1:min(end, len(s))], `\"`, `"`)
			s = s[min(end+1, len(s)):]
		} else {
			end := strings.IndexByte(s, ',')
			if end < 0 {
				end = len(s)
			}
			value = strings.TrimSpace(s[:end])
			s = s[end:]
		}
		out[key] = value
	}
	return out
}

// authorize computes the Authorization header answering the challenge.
func (c digestChallenge) authorize(method string, u *url.URL, username, password string, _ []byte) (string, error) {
	var newHash func() hash.Hash
	algorithm := strings.ToUpper(c.algorithm)
	session := strings.HasSuffix(algorithm, "-SESS")
	switch strings.TrimSuffix(algorithm, "-SESS") {
	case "", "MD5":
		newHash = md5.New
	case "SHA-256":
		newHash = sha256.New
	default:
		return "", fmt.Errorf("unsupported digest algorithm %q", c.algorithm)
	}
	h := func(s string) string {
		sum := newHash()
		sum.Write([]byte(s))
		return hex.EncodeToString(sum.Sum(nil))
	}
	cnonceBytes := make([]byte, 8)
	if _, err := rand.Read(cnonceBytes); err != nil {
		return "", err
	}
	cnonce := hex.EncodeToString(cnonceBytes)
	const nc = "00000001"

	uri := u.RequestURI()
	ha1 := h(username + ":" + c.realm + ":" + password)
	if session {
		ha1 = h(ha1 + ":" + c.nonce + ":" + cnonce)
	}
	ha2 := h(method + ":" + uri)
	var response string
	if c.qop == "auth" {
		response = h(strings.Join([]string{ha1, c.nonce, nc, cnonce, c.qop, ha2}, ":"))
	} else {
		response = h(ha1 + ":" + c.nonce + ":" + ha2)
	}

	parts := []string{
		fmt.Sprintf(`username="%s"`, quoteEscape(username)),
		fmt.Sprintf(`realm="%s"`, quoteEscape(c.realm)),
		fmt.Sprintf(`nonce="%s"`, quoteEscape(c.nonce)),
		fmt.Sprintf(`uri="%s"`, quoteEscape(uri)),
		fmt.Sprintf(`response="%s"`, response),
	}
	if c.algorithm != "" {
		parts = append(parts, "algorithm="+c.algorithm)
	}
	if c.opaque != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, quoteEscape(c.opaque)))
	}
	if c.qop != "" {
		parts = append(parts, "qop="+c.qop, "nc="+nc, fmt.Sprintf(`cnonce="%s"`, cnonce))
	}
	return "Digest " + strings.Join(parts, ", "), nil
}

func quoteEscape(s string) string { return strings.ReplaceAll(s, `"`, `\"`) }
