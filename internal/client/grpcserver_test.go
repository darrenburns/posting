package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bufbuild/protocompile"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// librarySchema compiles testdata/library.proto, so tests can serve it
// without generated code: every handler works on dynamicpb messages.
func librarySchema(t *testing.T) (*protoregistry.Files, protoreflect.ServiceDescriptor) {
	t.Helper()
	compiler := protocompile.Compiler{Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: []string{"testdata"}})}
	compiled, err := compiler.Compile(context.Background(), "library.proto")
	if err != nil {
		t.Fatal(err)
	}
	files := new(protoregistry.Files)
	if err := registerWithImports(files, compiled[0]); err != nil {
		t.Fatal(err)
	}
	return files, compiled[0].Services().ByName("Library")
}

type libraryOptions struct {
	noReflection bool
	// v1alphaOnly serves only the older reflection service.
	v1alphaOnly bool
	tls         bool
	// cert is the TLS server's certificate. Unset is a new self-signed one.
	cert tls.Certificate
}

// startLibrary serves the library service on a free loopback port and
// returns its address.
func startLibrary(t *testing.T, opts libraryOptions) string {
	t.Helper()
	files, svc := librarySchema(t)
	var serverOpts []grpc.ServerOption
	if opts.tls {
		cert := opts.cert
		if cert.Certificate == nil {
			cert = selfSigned(t)
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})))
	}
	server := grpc.NewServer(serverOpts...)
	server.RegisterService(libraryService(svc), struct{}{})
	if !opts.noReflection {
		ro := reflection.ServerOptions{Services: server, DescriptorResolver: files, ExtensionResolver: protoregistry.GlobalTypes}
		if !opts.v1alphaOnly {
			reflectionv1.RegisterServerReflectionServer(server, reflection.NewServerV1(ro))
		}
		reflectionv1alpha.RegisterServerReflectionServer(server, reflection.NewServer(ro))
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	return lis.Addr().String()
}

var libraryBooks = []string{
	`{"isbn": "1", "title": "Dune", "author": {"name": "Frank Herbert", "born": 1920}, "genre": "GENRE_FICTION", "published": "1965-08-01T00:00:00Z", "hardback": true}`,
	`{"isbn": "2", "title": "Dune Messiah", "author": {"name": "Frank Herbert", "born": 1920}, "genre": "GENRE_FICTION"}`,
	`{"isbn": "3", "title": "A Brief History of Time", "author": {"name": "Stephen Hawking", "born": 1942}, "genre": "GENRE_SCIENCE"}`,
}

func libraryService(svc protoreflect.ServiceDescriptor) *grpc.ServiceDesc {
	method := func(name string) protoreflect.MethodDescriptor { return svc.Methods().ByName(protoreflect.Name(name)) }
	field := func(m *dynamicpb.Message, name string) protoreflect.Value {
		return m.Get(m.Descriptor().Fields().ByName(protoreflect.Name(name)))
	}
	book := func(raw string) *dynamicpb.Message {
		m := dynamicpb.NewMessage(method("GetBook").Output())
		if err := protojson.Unmarshal([]byte(raw), m); err != nil {
			panic(err)
		}
		return m
	}
	return &grpc.ServiceDesc{
		ServiceName: string(svc.FullName()),
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "GetBook",
			Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				in := dynamicpb.NewMessage(method("GetBook").Input())
				if err := dec(in); err != nil {
					return nil, err
				}
				isbn := field(in, "isbn").String()
				if isbn != "hang" {
					echoMetadata(ctx)
				}
				for _, raw := range libraryBooks {
					if b := book(raw); field(b, "isbn").String() == isbn {
						_ = grpc.SetTrailer(ctx, metadata.Pairs("x-books-served", "1"))
						return b, nil
					}
				}
				if isbn == "hang" {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				if isbn == "detailed" {
					info, _ := anypb.New(&errdetails.ErrorInfo{Reason: "BOOK_MISSING", Domain: "library.test"})
					secret := &anypb.Any{TypeUrl: "type.googleapis.com/acme.Secret", Value: []byte{1, 2, 3}}
					p := status.New(codes.NotFound, "no book "+isbn).Proto()
					p.Details = append(p.Details, info, secret)
					return nil, status.ErrorProto(p)
				}
				return nil, status.Errorf(codes.NotFound, "no book %q", isbn)
			},
		}},
		Streams: []grpc.StreamDesc{
			{
				StreamName: "ListBooks", ServerStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					in := dynamicpb.NewMessage(method("ListBooks").Input())
					if err := stream.RecvMsg(in); err != nil {
						return err
					}
					echoMetadata(stream.Context())
					author := field(in, "author").String()
					if author == "forever" {
						for {
							if err := stream.SendMsg(book(libraryBooks[0])); err != nil {
								return err
							}
						}
					}
					sent := 0
					for _, raw := range libraryBooks {
						b := book(raw)
						name := field(b, "author").Message().Get(method("ListBooks").Output().Fields().ByName("author").Message().Fields().ByName("name")).String()
						if author != "" && author != "stall" && name != author {
							continue
						}
						if err := stream.SendMsg(b); err != nil {
							return err
						}
						sent++
						if author == "stall" && sent == 2 {
							<-stream.Context().Done()
							return stream.Context().Err()
						}
					}
					if author == "nobody" {
						return status.Error(codes.NotFound, "no books by nobody")
					}
					stream.SetTrailer(metadata.Pairs("x-books-served", fmt.Sprint(sent)))
					return nil
				},
			},
			{
				StreamName: "ShelveBooks", ClientStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					var titles []string
					for {
						in := dynamicpb.NewMessage(method("ShelveBooks").Input())
						err := stream.RecvMsg(in)
						if err == io.EOF {
							break
						}
						if err != nil {
							return err
						}
						titles = append(titles, field(in, "title").String())
					}
					out := dynamicpb.NewMessage(method("ShelveBooks").Output())
					data := fmt.Sprintf(`{"shelved": %d, "titles": ["%s"]}`, len(titles), strings.Join(titles, `", "`))
					if err := protojson.Unmarshal([]byte(data), out); err != nil {
						return err
					}
					return stream.SendMsg(out)
				},
			},
			{
				StreamName: "Chat", ServerStreams: true, ClientStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					for {
						in := dynamicpb.NewMessage(method("Chat").Input())
						err := stream.RecvMsg(in)
						if err == io.EOF {
							return nil
						}
						if err != nil {
							return err
						}
						out := dynamicpb.NewMessage(method("Chat").Output())
						out.Set(out.Descriptor().Fields().ByName("text"), protoreflect.ValueOfString(strings.ToUpper(field(in, "text").String())))
						if err := stream.SendMsg(out); err != nil {
							return err
						}
					}
				},
			},
		},
		Metadata: "library.proto",
	}
}

// echoMetadata sends each incoming x-* and authorization entry back as an
// echo-* header, so tests can see what metadata arrived.
func echoMetadata(ctx context.Context) {
	in, _ := metadata.FromIncomingContext(ctx)
	out := metadata.Pairs("x-library-branch", "central")
	for k, vs := range in {
		if strings.HasPrefix(k, "x-") || k == "authorization" {
			for _, v := range vs {
				out.Append("echo-"+k, v)
			}
		}
	}
	_ = grpc.SetHeader(ctx, out)
}

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
