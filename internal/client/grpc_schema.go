package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/darrenburns/posting/internal/model"
)

// ErrNoReflection is the error for a server that doesn't offer reflection,
// whose schema has to come from proto files instead.
var ErrNoReflection = errors.New("the server has no reflection service: add its proto files on the Proto tab")

// reflectionMethods are the reflection service's two versions, newest first.
// Their messages are the same on the wire, so v1's types serve both.
var reflectionMethods = []string{
	"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
	"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
}

// reflectFiles asks the server's reflection service for the files that
// define symbols, and the files they import. No symbols means every service
// the server lists.
func reflectFiles(ctx context.Context, conn *grpc.ClientConn, symbols []string) (*protoregistry.Files, error) {
	for _, method := range reflectionMethods {
		files, err := reflectWith(ctx, conn, method, symbols)
		if status.Code(err) == codes.Unimplemented {
			continue
		}
		return files, err
	}
	return nil, ErrNoReflection
}

func reflectWith(ctx context.Context, conn *grpc.ClientConn, method string, symbols []string) (*protoregistry.Files, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, method)
	if err != nil {
		return nil, err
	}
	ask := func(req *reflectionpb.ServerReflectionRequest) (*reflectionpb.ServerReflectionResponse, error) {
		// io.EOF means the server already ended the call, as one without
		// reflection may; RecvMsg has its status.
		if err := stream.SendMsg(req); err != nil && err != io.EOF {
			return nil, err
		}
		resp := new(reflectionpb.ServerReflectionResponse)
		if err := stream.RecvMsg(resp); err != nil {
			return nil, err
		}
		if e := resp.GetErrorResponse(); e != nil {
			return nil, status.Error(codes.Code(e.GetErrorCode()), e.GetErrorMessage())
		}
		return resp, nil
	}

	listed := len(symbols) == 0
	if listed {
		resp, err := ask(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{}})
		if err != nil {
			return nil, err
		}
		for _, service := range resp.GetListServicesResponse().GetService() {
			if !strings.HasPrefix(service.GetName(), "grpc.reflection.") {
				symbols = append(symbols, service.GetName())
			}
		}
	}

	protos := map[string]*descriptorpb.FileDescriptorProto{}
	add := func(resp *reflectionpb.ServerReflectionResponse) error {
		for _, raw := range resp.GetFileDescriptorResponse().GetFileDescriptorProto() {
			fd := new(descriptorpb.FileDescriptorProto)
			if err := proto.Unmarshal(raw, fd); err != nil {
				return fmt.Errorf("reading the server's schema: %w", err)
			}
			protos[fd.GetName()] = fd
		}
		return nil
	}
	// A listed service the server can't describe is left out, so the
	// others can still be listed.
	var unknown []string
	for _, symbol := range symbols {
		resp, err := ask(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol}})
		switch {
		case status.Code(err) == codes.NotFound && listed:
			unknown = append(unknown, symbol)
			continue
		case status.Code(err) == codes.NotFound:
			return nil, fmt.Errorf("the server doesn't know %s", symbol)
		case err != nil:
			return nil, err
		}
		if err := add(resp); err != nil {
			return nil, err
		}
	}
	if len(unknown) == len(symbols) && len(unknown) > 0 {
		return nil, fmt.Errorf("the server can't describe any of its services%s", listing("services", unknown))
	}
	// Servers usually send every import along with a file, but needn't.
	for {
		missing := missingImports(protos)
		if len(missing) == 0 {
			break
		}
		for _, name := range missing {
			if known, err := protoregistry.GlobalFiles.FindFileByPath(name); err == nil {
				protos[name] = protodesc.ToFileDescriptorProto(known)
				continue
			}
			resp, err := ask(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_FileByFilename{FileByFilename: name}})
			if err != nil {
				return nil, fmt.Errorf("fetching %s from the server: %w", name, err)
			}
			if err := add(resp); err != nil {
				return nil, err
			}
			if protos[name] == nil {
				return nil, fmt.Errorf("the server didn't send %s", name)
			}
		}
	}
	_ = stream.CloseSend()

	set := &descriptorpb.FileDescriptorSet{}
	for _, fd := range protos {
		set.File = append(set.File, fd)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, fmt.Errorf("reading the server's schema: %w", err)
	}
	return files, nil
}

func missingImports(protos map[string]*descriptorpb.FileDescriptorProto) []string {
	var missing []string
	for _, fd := range protos {
		for _, dep := range fd.GetDependency() {
			if protos[dep] == nil && !slices.Contains(missing, dep) {
				missing = append(missing, dep)
			}
		}
	}
	return missing
}

// protoCache keeps compiled proto sets. An entry is reused while every
// source it was built from has the size and modification time it had, so
// editing a .proto file is seen on the next send.
type protoCache struct {
	mu      sync.Mutex
	entries map[string]protoEntry
}

type protoEntry struct {
	sources []string
	stamp   string
	files   *protoregistry.Files
}

// load reads set, relative to root: .proto sources compiled with their
// imports (the standard ones built in), and descriptor sets as they are.
func (c *protoCache) load(root string, set model.ProtoSet) (*protoregistry.Files, error) {
	abs := set.Abs(root)
	files, imports := abs.Files, abs.ImportPaths
	key := strings.Join(files, "\x00") + "\x01" + strings.Join(imports, "\x00")
	c.mu.Lock()
	entry, ok := c.entries[key]
	c.mu.Unlock()
	if ok && stamp(entry.sources) == entry.stamp {
		return entry.files, nil
	}

	registry, sources, err := compileProtos(files, imports)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]protoEntry{}
	}
	c.entries[key] = protoEntry{sources: sources, stamp: stamp(sources), files: registry}
	c.mu.Unlock()
	return registry, nil
}

// compileProtos builds a registry from files, returning every file it read.
func compileProtos(files, imports []string) (*protoregistry.Files, []string, error) {
	registry := new(protoregistry.Files)
	var mu sync.Mutex
	var sources, names []string
	for _, path := range files {
		if isDescriptorSet(path) {
			if err := registerDescriptorSet(registry, path); err != nil {
				return nil, nil, err
			}
			sources = append(sources, path)
			continue
		}
		name, err := importName(path, imports)
		if err != nil {
			return nil, nil, err
		}
		names = append(names, name)
	}
	if len(names) > 0 {
		resolver := &protocompile.SourceResolver{
			ImportPaths: imports,
			Accessor: func(path string) (io.ReadCloser, error) {
				f, err := os.Open(path)
				if err == nil {
					mu.Lock()
					sources = append(sources, path)
					mu.Unlock()
				}
				return f, err
			},
		}
		compiler := protocompile.Compiler{Resolver: protocompile.WithStandardImports(resolver)}
		compiled, err := compiler.Compile(context.Background(), names...)
		if err != nil {
			return nil, nil, fmt.Errorf("compiling proto files: %w", err)
		}
		for _, fd := range compiled {
			if err := registerWithImports(registry, fd); err != nil {
				return nil, nil, err
			}
		}
	}
	sort.Strings(sources)
	return registry, slices.Compact(sources), nil
}

func isDescriptorSet(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".protoset", ".binpb", ".pb":
		return true
	}
	return false
}

func registerDescriptorSet(registry *protoregistry.Files, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	set := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(data, set); err != nil {
		return fmt.Errorf("%s isn't a descriptor set: %w", path, err)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var registerErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		registerErr = registerFile(registry, fd)
		return registerErr == nil
	})
	return registerErr
}

func registerWithImports(registry *protoregistry.Files, fd protoreflect.FileDescriptor) error {
	imports := fd.Imports()
	for i := range imports.Len() {
		if err := registerWithImports(registry, imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	return registerFile(registry, fd)
}

// registerFile adds fd unless a file of the same path is already there, as
// when two sets both carry a well-known type.
func registerFile(registry *protoregistry.Files, fd protoreflect.FileDescriptor) error {
	if _, err := registry.FindFileByPath(fd.Path()); err == nil {
		return nil
	}
	return registry.RegisterFile(fd)
}

// importName is path relative to the first import path it is under, which
// is the name protocompile finds it by.
func importName(path string, imports []string) (string, error) {
	for _, dir := range imports {
		if rel, err := filepath.Rel(dir, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel), nil
		}
	}
	return "", fmt.Errorf("%s isn't under any of the import paths", path)
}

// stamp identifies the current content of files by size and modification
// time.
func stamp(files []string) string {
	var b strings.Builder
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			b.WriteString(path + " missing\n")
			continue
		}
		fmt.Fprintf(&b, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
	}
	return b.String()
}

// schemaOf lists every method of every service in files.
func schemaOf(files *protoregistry.Files) Schema {
	var schema Schema
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := range services.Len() {
			service := services.Get(i)
			if strings.HasPrefix(string(service.FullName()), "grpc.reflection.") {
				continue
			}
			methods := service.Methods()
			for j := range methods.Len() {
				schema.Methods = append(schema.Methods, methodOf(methods.Get(j)))
			}
		}
		return true
	})
	sort.Slice(schema.Methods, func(i, j int) bool { return schema.Methods[i].Name < schema.Methods[j].Name })
	return schema
}

func methodOf(md protoreflect.MethodDescriptor) Method {
	m := Method{
		Name:      methodName(md),
		Streaming: streamingOf(md),
		Input:     string(md.Input().FullName()),
		Output:    string(md.Output().FullName()),
		Template:  template(md.Input()),
	}
	if m.Streaming.clientStreams() {
		m.Template = "[\n" + indent(m.Template) + "\n]"
	}
	return m
}

// methodName is "pkg.Service/Method".
func methodName(md protoreflect.MethodDescriptor) string {
	return string(md.Parent().FullName()) + "/" + string(md.Name())
}

func streamingOf(md protoreflect.MethodDescriptor) Streaming {
	switch {
	case md.IsStreamingClient() && md.IsStreamingServer():
		return BidiStream
	case md.IsStreamingClient():
		return ClientStream
	case md.IsStreamingServer():
		return ServerStream
	}
	return Unary
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}
