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
	"github.com/tomekjarosik/one-status/internal/web"
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
	cmd.Flags().String("http-port", "localhost:8088", "Address and port for HTTP gateway and Web UI to listen on")
	cmd.Flags().String("log-file", "server.log", "Path to log file")

	viper.BindPFlag("grpc_port", cmd.Flags().Lookup("grpc-port"))
	viper.BindPFlag("http_port", cmd.Flags().Lookup("http-port"))
	viper.BindPFlag("log_file", cmd.Flags().Lookup("log-file"))

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	grpcPort := viper.GetString("grpc_port")
	httpPort := viper.GetString("http_port")
	logFile := viper.GetString("log_file")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// 1. Initialize Storage & Service Layer
	storageSvc := storage.NewMemorySensorStorage()
	statussvc := server.NewStatusMonitorService(storageSvc)

	// 2. Initialize gRPC Server
	opts := []grpc.ServerOption{}
	if logFile != "" {
		opts = append(opts, grpc.UnaryInterceptor(middleware.LoggingInterceptor(logFile)))
	}

	grpcServer := grpc.NewServer(opts...)
	v1.RegisterStatusServiceServer(grpcServer, statussvc)

	go func() {
		l, err := net.Listen("tcp", grpcPort)
		if err != nil {
			grpclog.Fatalf("failed to listen: %v", err)
		}
		grpclog.Infof("gRPC server listening on %s", grpcPort)
		if err := grpcServer.Serve(l); err != nil && err != grpc.ErrServerStopped {
			grpclog.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// 3. Initialize HTTP Gateway (Translates HTTP/JSON -> gRPC)
	gateway, err := createGateway(ctx, grpcPort)
	if err != nil {
		return err
	}

	// 4. Initialize Web UI Dashboard
	dashboardHandler, err := web.NewDashboardHandler(statussvc)
	if err != nil {
		return err
	}

	// 5. Mount HTTP Routes
	mux := http.NewServeMux()
	mux.Handle("/api/", gateway) // Matches /api/ and all sub-paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Strict check for exactly "/" so it doesn't catch missing /api/ routes
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		dashboardHandler.ServeHTTP(w, r)
	})

	httpServer := &http.Server{Addr: httpPort, Handler: mux}

	go func() {
		grpclog.Infof("HTTP gateway and Web UI listening on %s", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			grpclog.Fatalf("HTTP server error: %v", err)
		}
	}()

	// 6. Wait for OS Shutdown Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	grpclog.Infof("Received signal %v, shutting down...", sig)

	// Trigger cancellation context for all background tasks
	cancel()

	// 7. Graceful Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	grpclog.Info("Shutting down HTTP server...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		grpclog.Errorf("HTTP server shutdown error: %v", err)
	}

	grpclog.Info("Shutting down gRPC server...")
	grpcServer.GracefulStop()

	grpclog.Info("Shutdown complete")
	return nil
}

// createGateway initializes the gRPC-Gateway reverse proxy
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
