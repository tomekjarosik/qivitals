package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/middleware"
	"github.com/tomekjarosik/one-status/internal/web"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

// App represents the composed gRPC + HTTP gateway + Web UI application.
type App struct {
	config    Config
	service   *StatusMonitorService // implements the gRPC service interface
	dashboard *web.DashboardHandler
}

// NewApp creates a new application instance.
func NewApp(cfg Config, svc *StatusMonitorService, dashboard *web.DashboardHandler) *App {
	return &App{
		config:    cfg,
		service:   svc,
		dashboard: dashboard,
	}
}

// Run starts all servers and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) error {
	grpcServer := a.newGRPCServer()
	grpcListener, err := net.Listen("tcp", a.config.GRPCPort)
	if err != nil {
		return err
	}

	gatewayHandler, err := a.newGatewayHandler(ctx)
	if err != nil {
		return err
	}

	// Assemble HTTP mux: /api/* -> gateway, / -> dashboard
	httpMux := http.NewServeMux()
	httpMux.Handle("/api/", http.StripPrefix("/api", gatewayHandler))
	httpMux.HandleFunc("/", a.dashboard.ServeHTTP)

	httpServer := &http.Server{
		Addr:    a.config.HTTPPort,
		Handler: httpMux,
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start servers in goroutines
	go func() {
		grpclog.Infof("gRPC server listening on %s", a.config.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil && err != grpc.ErrServerStopped {
			grpclog.Fatalf("gRPC server error: %v", err)
		}
	}()

	go func() {
		grpclog.Infof("HTTP gateway + UI listening on %s", a.config.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			grpclog.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	grpclog.Infof("Received signal %v, initiating graceful shutdown...", sig)

	// Create a timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server first (stops accepting new connections, finishes ongoing)
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		grpclog.Errorf("HTTP server shutdown error: %v", err)
	}

	// Then stop gRPC server gracefully
	grpcServer.GracefulStop()
	grpclog.Info("Shutdown complete")

	return nil
}

// newGRPCServer creates and configures the gRPC server with optional logging interceptor.
func (a *App) newGRPCServer() *grpc.Server {
	var opts []grpc.ServerOption
	if a.config.LogFile != "" {
		opts = append(opts, grpc.UnaryInterceptor(middleware.LoggingInterceptor(a.config.LogFile)))
	}
	grpcServer := grpc.NewServer(opts...)

	// Register the service implementation (the same instance that is used by the dashboard)
	v1.RegisterStatusServiceServer(grpcServer, a.service)

	return grpcServer
}

// newGatewayHandler creates a gRPC‑Gateway HTTP handler that forwards requests to the gRPC endpoint.
func (a *App) newGatewayHandler(ctx context.Context) (http.Handler, error) {
	mux := runtime.NewServeMux()
	conn, err := grpc.NewClient(
		a.config.GRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	if err := v1.RegisterStatusServiceHandler(ctx, mux, conn); err != nil {
		return nil, err
	}
	return mux, nil
}
