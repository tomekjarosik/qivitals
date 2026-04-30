package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestStatusServiceImpl_StartsWithoutPanic(t *testing.T) {
	impl := &StatusServiceImpl{}

	ctx := context.Background()

	_, err := impl.Echo(ctx, &v1.EchoRequest{
		Message: "test",
	})

	if err != nil {
		t.Fatalf("Echo failed: %v", err)
	}
}

func TestMockServerConnectivity(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	impl := &StatusServiceImpl{}
	v1.RegisterStatusServiceServer(grpcServer, impl)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	defer grpcServer.Stop()

	time.Sleep(10 * time.Millisecond)

	conn, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := v1.NewStatusServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := client.Echo(ctx, &v1.EchoRequest{
		Message: "test connectivity",
	})

	if err != nil {
		t.Fatalf("Failed to call Echo: %v", err)
	}

	if resp.Message != "test connectivity" || resp.Timestamp == 0 {
		t.Errorf("Unexpected response: %+v", resp)
	}
}
