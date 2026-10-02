// Package grpcurl writes gRPC requests as grpcurl commands.
package grpcurl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/model"
)

// FormatOptions control how Format writes a command.
type FormatOptions struct {
	// Multiline puts each option on its own line.
	Multiline bool
	// Root is the collection directory. Proto paths, which are relative to
	// it, are written as absolute paths so the command runs from anywhere.
	Root string
	// CACert, Cert and Key are the TLS settings' files: a CA bundle, and a
	// client certificate and its key. An empty Key means Cert holds both.
	CACert, Cert, Key string
}

// Format writes a gRPC request as a grpcurl command. Pass a resolved request
// (see model.Resolve) for a command that runs as-is.
func Format(req model.Request, opts FormatOptions) string {
	g, _ := req.Payload.(model.GRPC)
	var args [][]string
	address, tls := target(req.URL)
	switch {
	case !tls:
		args = append(args, []string{"-plaintext"})
	case !req.Options.VerifySSL:
		args = append(args, []string{"-insecure"})
	}
	if tls && opts.CACert != "" {
		args = append(args, []string{"-cacert", curl.Quote(opts.CACert)})
	}
	if tls && opts.Cert != "" {
		key := opts.Key
		if key == "" {
			key = opts.Cert
		}
		args = append(args, []string{"-cert", curl.Quote(opts.Cert)}, []string{"-key", curl.Quote(key)})
	}
	authorized := req.Auth.Type == model.AuthBasic || req.Auth.Type == model.AuthBearer
	for _, h := range req.Headers {
		name := strings.ToLower(strings.TrimSpace(h.Name))
		if h.Enabled && name != "" && !(authorized && name == "authorization") {
			args = append(args, []string{"-H", curl.Quote(name + ": " + h.Value)})
		}
	}
	switch req.Auth.Type {
	case model.AuthBasic:
		credentials := base64.StdEncoding.EncodeToString([]byte(req.Auth.Username + ":" + req.Auth.Password))
		args = append(args, []string{"-H", curl.Quote("authorization: Basic " + credentials)})
	case model.AuthBearer:
		args = append(args, []string{"-H", curl.Quote("authorization: Bearer " + req.Auth.Token)})
	}
	if data := messageData(g.Message); data != "" {
		args = append(args, []string{"-d", curl.Quote(data)})
	}
	if t := req.Options.TimeoutSeconds; t > 0 {
		args = append(args, []string{"-max-time", strconv.FormatFloat(t, 'f', -1, 64)})
	}
	args = append(args, protoArgs(g.Protos, opts.Root)...)
	args = append(args, []string{curl.Quote(address), curl.Quote(strings.TrimPrefix(strings.TrimSpace(g.Method), "/"))})

	parts := []string{"grpcurl"}
	for _, arg := range args {
		parts = append(parts, strings.Join(arg, " "))
	}
	if !opts.Multiline {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts, " \\\n  ")
}

// target is the address grpcurl takes, and whether to use TLS. An address
// with a variable left in is kept as written, without a guessed port, and
// only an explicit scheme says it is plaintext: grpcurl's default is TLS.
func target(url string) (address string, tls bool) {
	address = strings.TrimSpace(url)
	if len(model.FindVariables(address)) > 0 {
		if scheme, rest, ok := strings.Cut(address, "://"); ok {
			scheme = strings.ToLower(scheme)
			return rest, scheme != "grpc" && scheme != "http"
		}
		return address, true
	}
	if t, err := model.ParseGRPCTarget(address); err == nil {
		return t.Authority, t.TLS
	}
	return address, true
}

// messageData is the message as grpcurl's -d takes it. grpcurl reads a
// stream as JSON objects one after another, so an array's elements are
// written that way.
func messageData(message string) string {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "[") {
		return message
	}
	var items []json.RawMessage
	if json.Unmarshal([]byte(message), &items) != nil {
		return message
	}
	lines := make([]string, len(items))
	for i, item := range items {
		var compact bytes.Buffer
		if json.Compact(&compact, item) == nil {
			item = compact.Bytes()
		}
		lines[i] = string(item)
	}
	return strings.Join(lines, "\n")
}

// protoArgs are the flags for a proto set: descriptor sets as -protoset,
// sources as -proto named relative to the import paths, which default to
// the collection root as they do when Posting sends the request. Paths are
// made absolute first, as Posting does, so they match however written.
func protoArgs(set model.ProtoSet, root string) [][]string {
	if set.Reflection() {
		return nil
	}
	set = set.Abs(root)
	var args, sources [][]string
	for _, file := range set.Files {
		if model.IsDescriptorSet(file) {
			args = append(args, []string{"-protoset", curl.Quote(file)})
			continue
		}
		name, ok := model.ImportName(file, set.ImportPaths)
		if !ok {
			name = file
		}
		sources = append(sources, []string{"-proto", curl.Quote(name)})
	}
	if len(sources) == 0 {
		return args
	}
	for _, dir := range set.ImportPaths {
		args = append(args, []string{"-import-path", curl.Quote(dir)})
	}
	return append(args, sources...)
}
