package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/middleware"
	"github.com/tomekjarosik/one-status/internal/server"
	"github.com/tomekjarosik/one-status/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

func NewCmdServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Starts the gRPC and HTTP gateway servers",
		Long:  "Launches the status service monitoring system with gRPC and an HTTP gateway.",
		RunE:  runServe,
	}

	// Add flags and bind to viper
	cmd.Flags().String("grpc-port", "localhost:50051", "Address and port for gRPC server to listen on")
	cmd.Flags().String("http-port", "localhost:8088", "Address and port for HTTP gateway to listen on")
	cmd.Flags().String("log-file", "server.log", "Path to log file")

	viper.BindPFlag("grpc_port", cmd.Flags().Lookup("grpc-port"))
	viper.BindPFlag("http_port", cmd.Flags().Lookup("http-port"))
	viper.BindPFlag("log_file", cmd.Flags().Lookup("log-file"))

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	// Ensure viper reads from environment variables (like GRPC_PORT)
	viper.AutomaticEnv()

	grpcPort := viper.GetString("grpc_port")
	httpPort := viper.GetString("http_port")
	logFile := viper.GetString("log_file")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	storageSvc := storage.NewMemorySensorStorage()
	statussvc := server.NewStatusMonitorService(storageSvc)

	opts := []grpc.ServerOption{}
	if logFile != "" {
		opts = append(opts, grpc.UnaryInterceptor(middleware.LoggingInterceptor(logFile)))
	}

	grpcServer := grpc.NewServer(opts...)
	v1.RegisterStatusServiceServer(grpcServer, statussvc)


	// Start gRPC server in a goroutine
	go func() {
		l, err := net.Listen("tcp", grpcPort)
		if err != nil {
			grpclog.Fatalf("failed to listen: %v", err)
		}
		grpclog.Infof("gRPC server listening on %s", grpcPort)
		if err := grpcServer.Serve(l); err != nil && err != grpc.ErrServerStopped {
			grpclog.Fatalf("failed to serve: %v", err)
		}
	}()

	// Start HTTP gateway
	gatewayCtx, gatewayCancel := context.WithCancel(ctx)
	defer gatewayCancel()

	gateway, err := createGateway(gatewayCtx, grpcPort)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", gateway)

	httpServer := &http.Server{Addr: httpPort, Handler: mux}

	grpclog.Infof("HTTP gateway listening on %s", httpPort)

	go func() {
		<-ctx.Done()
		grpclog.Info("shutting down HTTP gateway...")
		gatewayCtxDone := gatewayCancel
		gatewayCtxDone()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			grpclog.Errorf("HTTP gateway shutdown error: %v", err)
		}
	}()

	// Listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	grpclog.Infof("received signal %v, shutting down...", sig)

	cancel()

	// Shutdown gRPC server
	_, grpcShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer grpcShutdownCancel()
	grpcServer.GracefulStop()
	grpclog.Info("gRPC server stopped")

	return nil
}

// createGateway now takes the configured grpcPort instead of relying on the constant
func createGateway(ctx context.Context, grpcPort string) (http.Handler, error) {
	mux := runtime.NewServeMux()

	conn, err := grpc.NewClient(
		grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024)),
	)
	if err != nil {
		return nil, err
	}

	if err := v1.RegisterStatusServiceHandler(ctx, mux, conn); err != nil {
		return nil, err
	}

	return mux, nil
}
