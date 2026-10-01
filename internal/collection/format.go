// Package collection reads and writes collections of requests on disk.
//
// A collection is a directory tree of Posting 2 request files
// (*.posting.yaml). Folders in the tree become child collections. The format
// is Posting 2's, so collections can be shared between both versions.
package collection

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/darrenburns/posting/internal/model"
)

// FileSuffix is the suffix of every request file.
const FileSuffix = ".posting.yaml"

// requestFile mirrors Posting 2's RequestModel. Field order is the order
// Posting 2 writes them in, and fields at their default value are left out,
// as Posting 2 does, so files stay short and diffs stay small. Kind and the
// kind's own block (graphql) are Posting 3's; both are absent for HTTP, and
// Posting 2 ignores them.
type requestFile struct {
	Name        yamlString      `yaml:"name,omitempty"`
	Description yamlString      `yaml:"description,omitempty"`
	Kind        string          `yaml:"kind,omitempty"`
	Method      string          `yaml:"method,omitempty"`
	URL         yamlString      `yaml:"url,omitempty"`
	GraphQL     *graphQLFile    `yaml:"graphql,omitempty"`
	Body        *bodyFile       `yaml:"body,omitempty"`
	Auth        *authFile       `yaml:"auth,omitempty"`
	Headers     []kvFile        `yaml:"headers,omitempty"`
	Params      []kvFile        `yaml:"params,omitempty"`
	PathParams  []pathParamFile `yaml:"path_params,omitempty"`
	Scripts     *scriptsFile    `yaml:"scripts,omitempty"`
	Options     *optionsFile    `yaml:"options,omitempty"`
}

type bodyFile struct {
	Content     *yamlString `yaml:"content,omitempty"`
	FormData    *[]kvFile   `yaml:"form_data,omitempty"`
	ContentType yamlString  `yaml:"content_type,omitempty"`
}

type authFile struct {
	Type        string           `yaml:"type,omitempty"`
	Basic       *credentialsFile `yaml:"basic,omitempty"`
	Digest      *credentialsFile `yaml:"digest,omitempty"`
	BearerToken *bearerTokenFile `yaml:"bearer_token,omitempty"`
}

type credentialsFile struct {
	Username yamlString `yaml:"username,omitempty"`
	Password yamlString `yaml:"password,omitempty"`
}

type bearerTokenFile struct {
	Token yamlString `yaml:"token,omitempty"`
}

// kvFile is a header, query parameter or form field. Enabled is only
// written when false, since Posting 2 defaults it to true.
type kvFile struct {
	Name    yamlString `yaml:"name"`
	Value   yamlString `yaml:"value"`
	Enabled *bool      `yaml:"enabled,omitempty"`
}

type pathParamFile struct {
	Name  yamlString `yaml:"name"`
	Value yamlString `yaml:"value"`
}

type scriptsFile struct {
	Setup      yamlString `yaml:"setup,omitempty"`
	OnRequest  yamlString `yaml:"on_request,omitempty"`
	OnResponse yamlString `yaml:"on_response,omitempty"`
}

// optionsFile uses pointers so only options that differ from the defaults
// are written, and missing options load as the defaults.
type optionsFile struct {
	FollowRedirects         *bool      `yaml:"follow_redirects,omitempty"`
	VerifySSL               *bool      `yaml:"verify_ssl,omitempty"`
	AttachCookies           *bool      `yaml:"attach_cookies,omitempty"`
	SubstituteBodyVariables *bool      `yaml:"substitute_body_variables,omitempty"`
	ProxyURL                yamlString `yaml:"proxy_url,omitempty"`
	Timeout                 *float64   `yaml:"timeout,omitempty"`
}

// ParseRequest decodes a request file. file is the collection-relative path
// recorded on the request.
func ParseRequest(data []byte, file string) (model.Request, error) {
	var in requestFile
	if err := yaml.Unmarshal(data, &in); err != nil {
		return model.Request{}, err
	}
	req := model.NewRequest()
	req.File = file
	req.Name = string(in.Name)
	req.Description = string(in.Description)
	req.URL = string(in.URL)
	if in.Method != "" {
		method := model.Method(strings.ToUpper(in.Method))
		if !validMethod(method) {
			return model.Request{}, fmt.Errorf("unsupported method %q", in.Method)
		}
		req.Method = method
	}
	req.Headers = kvsFromFile(in.Headers)
	req.Query = kvsFromFile(in.Params)
	for _, p := range in.PathParams {
		req.PathParams = append(req.PathParams, model.KeyValue{Name: string(p.Name), Value: string(p.Value), Enabled: true})
	}
	if b := in.Body; b != nil {
		switch {
		case b.FormData != nil:
			req.Body = model.Body{Type: model.BodyForm, Form: kvsFromFile(*b.FormData), ContentType: orDefault(string(b.ContentType), "application/x-www-form-urlencoded")}
		case b.Content != nil:
			req.Body = model.Body{Type: model.BodyRaw, Raw: string(*b.Content), ContentType: orDefault(string(b.ContentType), contentTypeFromHeaders(req.Headers))}
		}
	}
	if a := in.Auth; a != nil {
		switch model.AuthType(a.Type) {
		case model.AuthBasic:
			req.Auth = model.Auth{Type: model.AuthBasic}
			if a.Basic != nil {
				req.Auth.Username, req.Auth.Password = string(a.Basic.Username), string(a.Basic.Password)
			}
		case model.AuthDigest:
			req.Auth = model.Auth{Type: model.AuthDigest}
			if a.Digest != nil {
				req.Auth.Username, req.Auth.Password = string(a.Digest.Username), string(a.Digest.Password)
			}
		case model.AuthBearer:
			req.Auth = model.Auth{Type: model.AuthBearer}
			if a.BearerToken != nil {
				req.Auth.Token = string(a.BearerToken.Token)
			}
		}
	}
	if s := in.Scripts; s != nil {
		req.Scripts = model.Scripts{Setup: string(s.Setup), OnRequest: string(s.OnRequest), OnResponse: string(s.OnResponse)}
	}
	payload, err := decodePayload(in)
	if err != nil {
		return model.Request{}, err
	}
	req.Payload = payload
	if o := in.Options; o != nil {
		setBool(&req.Options.FollowRedirects, o.FollowRedirects)
		setBool(&req.Options.VerifySSL, o.VerifySSL)
		setBool(&req.Options.AttachCookies, o.AttachCookies)
		setBool(&req.Options.SubstituteBodyVariables, o.SubstituteBodyVariables)
		req.Options.ProxyURL = string(o.ProxyURL)
		if o.Timeout != nil && *o.Timeout > 0 {
			req.Options.TimeoutSeconds = *o.Timeout
		}
	}
	return model.Normalize(req), nil
}

// MarshalRequest encodes req in Posting 2's format.
func MarshalRequest(req model.Request) ([]byte, error) {
	req = model.Normalize(req)
	out := requestFile{
		Name:        yamlString(req.Name),
		Description: yamlString(req.Description),
		URL:         yamlString(req.URL),
		Headers:     kvsToFile(req.Headers),
		Params:      kvsToFile(req.Query),
	}
	// The editor keeps the enabled parameters in the URL as well as the
	// table. Files keep them only in the table, as Posting 2 writes them, so
	// they aren't listed twice.
	if len(req.Query) > 0 {
		// A question mark inside a fragment is not a query delimiter.
		base, fragment, hasFragment := strings.Cut(req.URL, "#")
		base, _, _ = strings.Cut(base, "?")
		out.URL = yamlString(base)
		if hasFragment {
			out.URL += yamlString("#" + fragment)
		}
	}
	if req.Method != model.MethodGet && req.Method != "" {
		out.Method = string(req.Method)
	}
	for _, p := range req.PathParams {
		out.PathParams = append(out.PathParams, pathParamFile{Name: yamlString(p.Name), Value: yamlString(p.Value)})
	}
	switch req.Body.Type {
	case model.BodyRaw:
		content := yamlString(req.Body.Raw)
		out.Body = &bodyFile{Content: &content, ContentType: yamlString(req.Body.ContentType)}
	case model.BodyForm:
		form := kvsToFile(req.Body.Form)
		if form == nil {
			form = []kvFile{}
		}
		out.Body = &bodyFile{FormData: &form, ContentType: "application/x-www-form-urlencoded"}
	}
	switch req.Auth.Type {
	case model.AuthBasic:
		out.Auth = &authFile{Type: string(model.AuthBasic), Basic: &credentialsFile{Username: yamlString(req.Auth.Username), Password: yamlString(req.Auth.Password)}}
	case model.AuthDigest:
		out.Auth = &authFile{Type: string(model.AuthDigest), Digest: &credentialsFile{Username: yamlString(req.Auth.Username), Password: yamlString(req.Auth.Password)}}
	case model.AuthBearer:
		out.Auth = &authFile{Type: string(model.AuthBearer), BearerToken: &bearerTokenFile{Token: yamlString(req.Auth.Token)}}
	}
	if s := req.Scripts; s != (model.Scripts{}) {
		out.Scripts = &scriptsFile{Setup: yamlString(s.Setup), OnRequest: yamlString(s.OnRequest), OnResponse: yamlString(s.OnResponse)}
	}
	defaults := model.DefaultOptions()
	opts := optionsFile{ProxyURL: yamlString(req.Options.ProxyURL)}
	if req.Options.FollowRedirects != defaults.FollowRedirects {
		opts.FollowRedirects = boolPtr(req.Options.FollowRedirects)
	}
	if req.Options.VerifySSL != defaults.VerifySSL {
		opts.VerifySSL = boolPtr(req.Options.VerifySSL)
	}
	if req.Options.AttachCookies != defaults.AttachCookies {
		opts.AttachCookies = boolPtr(req.Options.AttachCookies)
	}
	if req.Options.SubstituteBodyVariables != defaults.SubstituteBodyVariables {
		opts.SubstituteBodyVariables = boolPtr(req.Options.SubstituteBodyVariables)
	}
	if req.Options.TimeoutSeconds != defaults.TimeoutSeconds && req.Options.TimeoutSeconds > 0 {
		timeout := req.Options.TimeoutSeconds
		opts.Timeout = &timeout
	}
	if opts != (optionsFile{}) {
		out.Options = &opts
	}
	if err := encodePayload(req, &out); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(out); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// yamlString prevents yaml.v3 from emitting an invalid block scalar for
// leading-newline and tab-leading multiline strings. Other strings keep the
// normal readable style.
type yamlString string

func (s yamlString) MarshalYAML() (any, error) {
	value := string(s)
	if utf8.ValidString(value) && (strings.HasPrefix(value, "\n") || (strings.Contains(value, "\t") && strings.ContainsAny(value, "\n\r")) || (value != "" && strings.TrimSpace(value) == "")) {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}, nil
	}
	// Let the encoder choose ordinary block scalars and !!binary for strings
	// with invalid UTF-8, preserving arbitrary payload bytes.
	return value, nil
}

func kvsFromFile(in []kvFile) []model.KeyValue {
	var out []model.KeyValue
	for _, kv := range in {
		enabled := kv.Enabled == nil || *kv.Enabled
		out = append(out, model.KeyValue{Name: string(kv.Name), Value: string(kv.Value), Enabled: enabled})
	}
	return out
}

func kvsToFile(in []model.KeyValue) []kvFile {
	var out []kvFile
	for _, kv := range in {
		item := kvFile{Name: yamlString(kv.Name), Value: yamlString(kv.Value)}
		if !kv.Enabled {
			item.Enabled = boolPtr(false)
		}
		out = append(out, item)
	}
	return out
}

// contentTypeFromHeaders guesses a raw body's type from its Content-Type
// header, for files that don't record one on the body.
func contentTypeFromHeaders(headers []model.KeyValue) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, "Content-Type") && h.Enabled {
			ct, _, _ := strings.Cut(h.Value, ";")
			return strings.TrimSpace(ct)
		}
	}
	return "application/json"
}

func validMethod(m model.Method) bool {
	for _, candidate := range model.Methods {
		if m == candidate {
			return true
		}
	}
	return false
}

func setBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

func boolPtr(b bool) *bool { return &b }

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
