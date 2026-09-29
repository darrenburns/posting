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

	"gopkg.in/yaml.v3"

	"github.com/darrenburns/posting/internal/model"
)

// FileSuffix is the suffix of every request file.
const FileSuffix = ".posting.yaml"

// requestFile mirrors Posting 2's RequestModel. Field order is the order
// Posting 2 writes them in, and fields at their default value are left out,
// as Posting 2 does, so files stay short and diffs stay small.
type requestFile struct {
	Name        string          `yaml:"name,omitempty"`
	Description string          `yaml:"description,omitempty"`
	Method      string          `yaml:"method,omitempty"`
	URL         string          `yaml:"url,omitempty"`
	Body        *bodyFile       `yaml:"body,omitempty"`
	Auth        *authFile       `yaml:"auth,omitempty"`
	Headers     []kvFile        `yaml:"headers,omitempty"`
	Params      []kvFile        `yaml:"params,omitempty"`
	PathParams  []pathParamFile `yaml:"path_params,omitempty"`
	Scripts     *scriptsFile    `yaml:"scripts,omitempty"`
	Options     *optionsFile    `yaml:"options,omitempty"`
}

type bodyFile struct {
	Content     *string   `yaml:"content,omitempty"`
	FormData    *[]kvFile `yaml:"form_data,omitempty"`
	ContentType string    `yaml:"content_type,omitempty"`
}

type authFile struct {
	Type        string           `yaml:"type,omitempty"`
	Basic       *credentialsFile `yaml:"basic,omitempty"`
	Digest      *credentialsFile `yaml:"digest,omitempty"`
	BearerToken *bearerTokenFile `yaml:"bearer_token,omitempty"`
}

type credentialsFile struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type bearerTokenFile struct {
	Token string `yaml:"token,omitempty"`
}

// kvFile is a header, query parameter or form field. Enabled is only
// written when false, since Posting 2 defaults it to true.
type kvFile struct {
	Name    string `yaml:"name"`
	Value   string `yaml:"value"`
	Enabled *bool  `yaml:"enabled,omitempty"`
}

type pathParamFile struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type scriptsFile struct {
	Setup      string `yaml:"setup,omitempty"`
	OnRequest  string `yaml:"on_request,omitempty"`
	OnResponse string `yaml:"on_response,omitempty"`
}

// optionsFile uses pointers so only options that differ from the defaults
// are written, and missing options load as the defaults.
type optionsFile struct {
	FollowRedirects         *bool    `yaml:"follow_redirects,omitempty"`
	VerifySSL               *bool    `yaml:"verify_ssl,omitempty"`
	AttachCookies           *bool    `yaml:"attach_cookies,omitempty"`
	SubstituteBodyVariables *bool    `yaml:"substitute_body_variables,omitempty"`
	ProxyURL                string   `yaml:"proxy_url,omitempty"`
	Timeout                 *float64 `yaml:"timeout,omitempty"`
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
	req.Name = in.Name
	req.Description = in.Description
	req.URL = in.URL
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
		req.PathParams = append(req.PathParams, model.KeyValue{Name: p.Name, Value: p.Value, Enabled: true})
	}
	if b := in.Body; b != nil {
		switch {
		case b.FormData != nil:
			req.Body = model.Body{Type: model.BodyForm, Form: kvsFromFile(*b.FormData), ContentType: orDefault(b.ContentType, "application/x-www-form-urlencoded")}
		case b.Content != nil:
			req.Body = model.Body{Type: model.BodyRaw, Raw: *b.Content, ContentType: orDefault(b.ContentType, contentTypeFromHeaders(req.Headers))}
		}
	}
	if a := in.Auth; a != nil {
		switch model.AuthType(a.Type) {
		case model.AuthBasic:
			req.Auth = model.Auth{Type: model.AuthBasic}
			if a.Basic != nil {
				req.Auth.Username, req.Auth.Password = a.Basic.Username, a.Basic.Password
			}
		case model.AuthDigest:
			req.Auth = model.Auth{Type: model.AuthDigest}
			if a.Digest != nil {
				req.Auth.Username, req.Auth.Password = a.Digest.Username, a.Digest.Password
			}
		case model.AuthBearer:
			req.Auth = model.Auth{Type: model.AuthBearer}
			if a.BearerToken != nil {
				req.Auth.Token = a.BearerToken.Token
			}
		}
	}
	if s := in.Scripts; s != nil {
		req.Scripts = model.Scripts{Setup: s.Setup, OnRequest: s.OnRequest, OnResponse: s.OnResponse}
	}
	if o := in.Options; o != nil {
		setBool(&req.Options.FollowRedirects, o.FollowRedirects)
		setBool(&req.Options.VerifySSL, o.VerifySSL)
		setBool(&req.Options.AttachCookies, o.AttachCookies)
		setBool(&req.Options.SubstituteBodyVariables, o.SubstituteBodyVariables)
		req.Options.ProxyURL = o.ProxyURL
		if o.Timeout != nil && *o.Timeout > 0 {
			req.Options.TimeoutSeconds = *o.Timeout
		}
	}
	return req, nil
}

// MarshalRequest encodes req in Posting 2's format.
func MarshalRequest(req model.Request) ([]byte, error) {
	out := requestFile{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
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
		out.URL = base
		if hasFragment {
			out.URL += "#" + fragment
		}
	}
	if req.Method != model.MethodGet && req.Method != "" {
		out.Method = string(req.Method)
	}
	for _, p := range req.PathParams {
		out.PathParams = append(out.PathParams, pathParamFile{Name: p.Name, Value: p.Value})
	}
	switch req.Body.Type {
	case model.BodyRaw:
		content := req.Body.Raw
		out.Body = &bodyFile{Content: &content, ContentType: req.Body.ContentType}
	case model.BodyForm:
		form := kvsToFile(req.Body.Form)
		if form == nil {
			form = []kvFile{}
		}
		out.Body = &bodyFile{FormData: &form, ContentType: "application/x-www-form-urlencoded"}
	}
	switch req.Auth.Type {
	case model.AuthBasic:
		out.Auth = &authFile{Type: string(model.AuthBasic), Basic: &credentialsFile{Username: req.Auth.Username, Password: req.Auth.Password}}
	case model.AuthDigest:
		out.Auth = &authFile{Type: string(model.AuthDigest), Digest: &credentialsFile{Username: req.Auth.Username, Password: req.Auth.Password}}
	case model.AuthBearer:
		out.Auth = &authFile{Type: string(model.AuthBearer), BearerToken: &bearerTokenFile{Token: req.Auth.Token}}
	}
	if s := req.Scripts; s != (model.Scripts{}) {
		out.Scripts = &scriptsFile{Setup: s.Setup, OnRequest: s.OnRequest, OnResponse: s.OnResponse}
	}
	defaults := model.DefaultOptions()
	opts := optionsFile{ProxyURL: req.Options.ProxyURL}
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

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(trimLineEnds(out)); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// trimLineEnds strips trailing spaces from the lines of multi-line strings.
// YAML can only write a multi-line string as a readable block when no line
// ends in a space; Posting 2 strips them for the same reason.
func trimLineEnds(r requestFile) requestFile {
	trim := func(s string) string {
		if !strings.Contains(s, "\n") {
			return s
		}
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		return strings.Join(lines, "\n")
	}
	r.Description = trim(r.Description)
	if r.Body != nil && r.Body.Content != nil {
		body := *r.Body
		content := trim(*body.Content)
		body.Content = &content
		r.Body = &body
	}
	return r
}

func kvsFromFile(in []kvFile) []model.KeyValue {
	var out []model.KeyValue
	for _, kv := range in {
		enabled := kv.Enabled == nil || *kv.Enabled
		out = append(out, model.KeyValue{Name: kv.Name, Value: kv.Value, Enabled: enabled})
	}
	return out
}

func kvsToFile(in []model.KeyValue) []kvFile {
	var out []kvFile
	for _, kv := range in {
		item := kvFile{Name: kv.Name, Value: kv.Value}
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
