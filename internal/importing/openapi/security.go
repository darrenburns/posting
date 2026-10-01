package openapi

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

func keys(m object) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (p *parser) security(r *model.Request, raw any, context string) {
	if raw == nil {
		return
	}
	requirements, ok := raw.([]any)
	if !ok {
		p.fail("%s: security must be an array", context)
		return
	}
	if len(requirements) == 0 {
		return
	}
	for _, item := range requirements {
		requirement, ok := item.(object)
		if !ok {
			p.fail("%s: security requirement must be an object", context)
			return
		}
		if len(requirement) == 0 {
			if len(requirements) > 1 {
				p.warn("%s: selected anonymous access from security alternatives", context)
			}
			return
		}
	}
	schemes := obj(obj(p.root["components"])["securitySchemes"])
	// Choose a complete representable OR alternative; never silently import only
	// part of an AND requirement. Multiple HTTP schemes cannot share Authorization.
	for index, item := range requirements {
		requirement := obj(item)
		resolved := map[string]object{}
		supported := true
		authCount := 0
		for _, name := range keys(requirement) {
			scheme := p.resolve(schemes[name], context+" security scheme "+name)
			resolved[name] = scheme
			switch str(scheme["type"]) {
			case "http":
				if strings.ToLower(str(scheme["scheme"])) != "basic" && strings.ToLower(str(scheme["scheme"])) != "bearer" && strings.ToLower(str(scheme["scheme"])) != "digest" {
					supported = false
				}
				authCount++
			case "apiKey":
				in := str(scheme["in"])
				if str(scheme["name"]) == "" || (in != "header" && in != "query" && in != "cookie") {
					supported = false
				}
				if in == "header" && strings.EqualFold(str(scheme["name"]), "Authorization") {
					authCount++
				}
			default:
				supported = false
			}
		}
		if authCount > 1 {
			supported = false
		}
		if !supported {
			continue
		}
		if len(requirements) > 1 {
			p.warn("%s: selected security alternative %d of %d", context, index+1, len(requirements))
		}
		for _, name := range keys(requirement) {
			scheme := resolved[name]
			prefix := authPrefix(name)
			if str(scheme["type"]) == "http" {
				switch strings.ToLower(str(scheme["scheme"])) {
				case "basic", "digest":
					typ := model.AuthBasic
					if strings.ToLower(str(scheme["scheme"])) == "digest" {
						typ = model.AuthDigest
					}
					r.Auth = model.Auth{Type: typ, Username: p.placeholder(prefix + "_USERNAME"), Password: p.placeholder(prefix + "_PASSWORD")}
				case "bearer":
					r.Auth = model.Auth{Type: model.AuthBearer, Token: p.placeholder(prefix + "_TOKEN")}
				}
			} else {
				value := p.placeholder(prefix + "_KEY")
				key := literal(str(scheme["name"]))
				switch str(scheme["in"]) {
				case "header":
					r.Headers = append(r.Headers, model.KeyValue{Name: key, Value: value, Enabled: true})
				case "query":
					r.Query = append(r.Query, model.KeyValue{Name: key, Value: value, Enabled: true})
				case "cookie":
					p.appendCookie(r, key+"="+value)
				}
			}
		}
		p.warn("%s: authentication credentials are empty placeholders; configure imported variables before sending", context)
		return
	}
	p.warn("%s: security requirements cannot be represented (OAuth/OpenID/mTLS or incompatible schemes); authentication needs manual configuration", context)
}

func authPrefix(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	// Keep readable names bounded while preserving a suffix for collision resistance.
	label := b.String()
	if len(label) > 48 {
		label = label[:48]
	}
	digest := sha256.Sum256([]byte(name))
	return fmt.Sprintf("OPENAPI_%s_%x", label, digest[:4])
}
func (p *parser) addVariable(name, value string) {
	if !p.variableNames[name] {
		p.outputBytes += len(name) + len(value) + len("OpenAPI")
		if p.outputBytes > maxMaterializedBytes {
			p.fail("OpenAPI materialized request data exceeds 32 MiB")
			return
		}
		p.variableNames[name] = true
		p.result.Variables = append(p.result.Variables, model.Variable{Name: name, Value: literal(value), Source: "OpenAPI"})
	}
}
func (p *parser) placeholder(name string) string { p.addVariable(name, ""); return "${" + name + "}" }
func (p *parser) cookie(r *model.Request, name, value string) {
	p.appendCookie(r, literal(url.QueryEscape(name)+"="+url.QueryEscape(value)))
}
func (p *parser) appendCookie(r *model.Request, value string) {
	for i, h := range r.Headers {
		if strings.EqualFold(h.Name, "Cookie") && h.Enabled {
			r.Headers[i].Value += "; " + value
			return
		}
	}
	r.Headers = append(r.Headers, model.KeyValue{Name: "Cookie", Value: value, Enabled: true})
}
