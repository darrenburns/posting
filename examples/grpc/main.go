// A local gRPC playground with all four RPC shapes and server reflection.
// Run from the repository root: go run ./examples/grpc
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/bufbuild/protocompile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Compile the embedded schema at startup instead of requiring protoc or
// checking generated code into this small example.
//
//go:embed demo.proto
var schema string

func main() {
	address := flag.String("addr", "127.0.0.1:50051", "plaintext listen address (local demo only)")
	flag.Parse()
	server, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("gRPC demo listening on %s (plaintext, reflection enabled)", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func newServer() (*grpc.Server, error) {
	compiler := protocompile.Compiler{Resolver: &protocompile.SourceResolver{
		Accessor: protocompile.SourceAccessorFromMap(map[string]string{"demo.proto": schema}),
	}}
	files, err := compiler.Compile(context.Background(), "demo.proto")
	if err != nil {
		return nil, err
	}
	registry := new(protoregistry.Files)
	if err := registry.RegisterFile(files[0]); err != nil {
		return nil, err
	}
	service := files[0].Services().ByName("Greeter")
	method := func(name string) protoreflect.MethodDescriptor {
		return service.Methods().ByName(protoreflect.Name(name))
	}
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: string(service.FullName()),
		HandlerType: (*any)(nil),
		Metadata:    "demo.proto",
		Methods: []grpc.MethodDesc{{
			MethodName: "SayHello",
			Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				m := method("SayHello")
				in := dynamicpb.NewMessage(m.Input())
				if err := decode(in); err != nil {
					return nil, err
				}
				text := "Hello, " + value(in, "name") + "!"
				log.Printf("SayHello: %s", text)
				_ = grpc.SetHeader(ctx, metadata.Pairs("x-demo-server", "posting-grpc"))
				_ = grpc.SetTrailer(ctx, metadata.Pairs("x-messages-sent", "1"))
				return reply(m, text, 1), nil
			},
		}},
		Streams: []grpc.StreamDesc{
			{
				StreamName: "StreamGreetings", ServerStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					m := method("StreamGreetings")
					in := dynamicpb.NewMessage(m.Input())
					if err := stream.RecvMsg(in); err != nil {
						return err
					}
					for i, greeting := range []string{"Hello", "Bonjour", "Hola"} {
						if i > 0 {
							select {
							case <-time.After(time.Second):
							case <-stream.Context().Done():
								return stream.Context().Err()
							}
						}
						text := greeting + ", " + value(in, "name") + "!"
						if err := stream.SendMsg(reply(m, text, int32(i+1))); err != nil {
							return err
						}
						log.Printf("StreamGreetings: reply %d: %s", i+1, text)
					}
					stream.SetTrailer(metadata.Pairs("x-messages-sent", "3"))
					return nil
				},
			},
			{
				StreamName: "CollectNames", ClientStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					m := method("CollectNames")
					out := dynamicpb.NewMessage(m.Output())
					names := out.Mutable(out.Descriptor().Fields().ByName("names")).List()
					for {
						in := dynamicpb.NewMessage(m.Input())
						if err := stream.RecvMsg(in); err == io.EOF {
							break
						} else if err != nil {
							return err
						}
						name := value(in, "name")
						names.Append(protoreflect.ValueOfString(name))
						log.Printf("CollectNames: received %q", name)
					}
					out.Set(out.Descriptor().Fields().ByName("count"), protoreflect.ValueOfInt32(int32(names.Len())))
					stream.SetTrailer(metadata.Pairs("x-names-received", fmt.Sprint(names.Len())))
					log.Printf("CollectNames: client half-closed; returning %d names", names.Len())
					return stream.SendMsg(out)
				},
			},
			{
				StreamName: "Chat", ClientStreams: true, ServerStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					m := method("Chat")
					log.Print("Chat: stream opened")
					var sequence int32
					defer func() { log.Printf("Chat: stream ended after %d replies", sequence) }()
					for {
						in := dynamicpb.NewMessage(m.Input())
						if err := stream.RecvMsg(in); err == io.EOF {
							log.Print("Chat: client half-closed")
							stream.SetTrailer(metadata.Pairs("x-messages-sent", fmt.Sprint(sequence)))
							return nil
						} else if err != nil {
							return err
						}
						sequence++
						text := value(in, "text")
						log.Printf("Chat: received %d: %q", sequence, text)
						if err := stream.SendMsg(reply(m, strings.ToUpper(text), sequence)); err != nil {
							return err
						}
						log.Printf("Chat: sent reply %d (client can send again)", sequence)
					}
				},
			},
		},
	}, struct{}{})
	options := reflection.ServerOptions{
		Services: server, DescriptorResolver: registry,
	}
	reflectionv1.RegisterServerReflectionServer(server, reflection.NewServerV1(options))
	reflectionv1alpha.RegisterServerReflectionServer(server, reflection.NewServer(options))
	return server, nil
}

func value(message *dynamicpb.Message, name protoreflect.Name) string {
	return message.Get(message.Descriptor().Fields().ByName(name)).String()
}

func reply(method protoreflect.MethodDescriptor, text string, sequence int32) *dynamicpb.Message {
	message := dynamicpb.NewMessage(method.Output())
	message.Set(message.Descriptor().Fields().ByName("text"), protoreflect.ValueOfString(text))
	message.Set(message.Descriptor().Fields().ByName("sequence"), protoreflect.ValueOfInt32(sequence))
	return message
}
