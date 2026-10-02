// Package grpcurl writes gRPC requests as grpcurl commands.
package grpcurl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
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
	address := strings.TrimSpace(req.URL)
	if target, err := model.ParseGRPCTarget(req.URL); err == nil {
		address = target.Authority
		switch {
		case !target.TLS:
			args = append(args, []string{"-plaintext"})
		case !req.Options.VerifySSL:
			args = append(args, []string{"-insecure"})
		}
	}
	for _, h := range req.Headers {
		if h.Enabled && strings.TrimSpace(h.Name) != "" {
			args = append(args, []string{"-H", curl.Quote(strings.ToLower(strings.TrimSpace(h.Name)) + ": " + h.Value)})
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
// the collection root as they do when Posting sends the request.
func protoArgs(set model.ProtoSet, root string) [][]string {
	if set.Reflection() {
		return nil
	}
	abs := func(path string) string {
		if root == "" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	imports := set.ImportPaths
	if len(imports) == 0 {
		imports = []string{"."}
	}
	var args, sources [][]string
	for _, file := range set.Files {
		switch strings.ToLower(filepath.Ext(file)) {
		case ".protoset", ".binpb", ".pb":
			args = append(args, []string{"-protoset", curl.Quote(abs(file))})
		default:
			sources = append(sources, []string{"-proto", curl.Quote(importName(file, imports))})
		}
	}
	if len(sources) == 0 {
		return args
	}
	for _, dir := range imports {
		args = append(args, []string{"-import-path", curl.Quote(abs(dir))})
	}
	return append(args, sources...)
}

// importName is file relative to the first import path it is under.
func importName(file string, imports []string) string {
	for _, dir := range imports {
		if rel, err := filepath.Rel(dir, file); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return file
}
