package client

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/darrenburns/posting/v3/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestGRPCEmptyMessagesRespectResponseCap(t *testing.T) {
	_, svc := librarySchema(t)
	md := svc.Methods().ByName("ListBooks")
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: string(svc.FullName()), HandlerType: (*any)(nil),
		Streams: []grpc.StreamDesc{{StreamName: "ListBooks", ServerStreams: true, Handler: func(_ any, stream grpc.ServerStream) error {
			if err := stream.RecvMsg(dynamicpb.NewMessage(md.Input())); err != nil {
				return err
			}
			for range 1000 {
				if err := stream.SendMsg(dynamicpb.NewMessage(md.Output())); err != nil {
					return err
				}
			}
			return nil
		}}},
	}, struct{}{})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(lis)
	t.Cleanup(server.Stop)
	req := grpcRequest(lis.Addr().String(), "library.v1.Library/ListBooks", "{}")
	p := req.Payload.(model.GRPC)
	p.Protos = model.ProtoSet{Files: []string{"library.proto"}}
	req.Payload = p
	g := NewGRPC("", TLSSettings{}, "testdata")
	g.maxResponse = 256
	resp, err := g.Send(context.Background(), Call{Request: req})
	if err != nil {
		t.Fatal(err)
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(resp.Body, &messages); err != nil {
		t.Fatal(err)
	}
	if resp.GRPC.Code != 1 || !strings.Contains(resp.GRPC.Message, "response truncated") {
		t.Fatalf("256-byte cap retained %d messages / %d JSON bytes, status=%+v", len(messages), len(resp.Body), resp.GRPC)
	}
	if len(messages) == 0 || len(resp.Body) > g.maxResponse {
		t.Fatalf("retained %d messages in %d bytes, want a nonempty body within %d bytes", len(messages), len(resp.Body), g.maxResponse)
	}
}
