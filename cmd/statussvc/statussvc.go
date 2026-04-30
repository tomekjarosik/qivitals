package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"

	"github.com/tomekjarosik/one-status/internal/server"
	"github.com/tomekjarosik/one-status/internal/storage"
)

const (
	grpcPort = "localhost:50051"
	httpPort = "localhost:8080"
)

func main() {
	grpcServer := grpc.NewServer()

	storage := storage.NewMemorySensorStorage()
	statussvc := server.NewStatusMonitorService(storage)
	v1.RegisterStatusServiceServer(grpcServer, statussvc)

	// Start gRPC server
	go func() {
		l, err := net.Listen("tcp", grpcPort)
		if err != nil {
			grpclog.Fatalf("failed to listen: %v", err)
		}
		grpclog.Infof("gRPC server listening on %s", grpcPort)
		if err := grpcServer.Serve(l); err != nil {
			grpclog.Fatalf("failed to serve: %v", err)
		}
	}()

	// Start HTTP gateway
	gateway := createGateway()
	mux := http.NewServeMux()
	mux.Handle("/api/", gateway)

	grpclog.Infof("HTTP gateway listening on %s", httpPort)
	if err := http.ListenAndServe(httpPort, mux); err != nil {
		log.Fatalf("failed to serve HTTP gateway: %v", err)
	}
}

func createGateway() http.Handler {
	ctx := context.Background()
	mux := runtime.NewServeMux()

	conn, err := grpc.NewClient(
		grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024)),
	)
	if err != nil {
		grpclog.Fatalf("Failed to dial server: %v", err)
	}

	if err := v1.RegisterStatusServiceHandler(ctx, mux, conn); err != nil {
		grpclog.Fatalf("Failed to register handler: %v", err)
	}

	return mux
}
