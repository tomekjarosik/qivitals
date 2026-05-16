package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

// App represents the composed gRPC + HTTP gateway + Web UI application.
type App struct {
	config     Config
	service    *QiVitalsService
	webHandler http.Handler

	authenticator *auth.Authenticator
}

func NewApp(cfg Config, svc *QiVitalsService, webHandler http.Handler, authenticator *auth.Authenticator) *App {
	return &App{
		config:        cfg,
		service:       svc,
		webHandler:    webHandler,
		authenticator: authenticator,
	}
}

func (a *App) Run(ctx context.Context) error {
	var logger *slog.Logger
	var cleanup func()

	if a.config.LogFile != "" {
		logFile, err := os.OpenFile(a.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			AddSource: true,
		})
		logger = slog.New(fileHandler)

		// Define cleanup to run at the end of Run()
		cleanup = func() {
			logFile.Close()
		}
	} else {
		consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		})
		logger = slog.New(consoleHandler)
	}

	// Ensure file is closed when Run finishes (whether successful or on error)
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	grpcServer := a.newGRPCServer(logger)
	grpcListener, err := net.Listen("tcp", a.config.GRPCPort)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:    a.config.HTTPPort,
		Handler: a.webHandler,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC
	go func() {
		grpclog.Infof("gRPC server listening on %s", a.config.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Start HTTP
	go func() {
		grpclog.Infof("HTTP gateway + UI listening on %s", a.config.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown
	sig := <-sigCh
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	grpcServer.GracefulStop()
	log.Println("Shutdown complete")

	return nil
}

// newGRPCServer creates and configures the gRPC server with optional logging interceptor.
func (a *App) newGRPCServer(logger *slog.Logger) *grpc.Server {
	authInterceptor := auth.ServerInterceptor(a.authenticator)
	interceptors := make([]grpc.UnaryServerInterceptor, 0)
	interceptors = append(interceptors, middleware.LoggingInterceptor(logger))
	interceptors = append(interceptors, authInterceptor)
	var opts []grpc.ServerOption
	opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	grpcServer := grpc.NewServer(opts...)

	// Register the service implementation (the same instance that is used by the dashboard)
	v1.RegisterQiVitalsServiceServer(grpcServer, a.service)

	return grpcServer
}

// NewGatewayHandler creates a gRPC‑Gateway HTTP handler that forwards requests to the gRPC endpoint.
func NewGatewayHandler(ctx context.Context, grpcPort string) (http.Handler, v1.QiVitalsServiceClient, error) {
	mux := runtime.NewServeMux(
		runtime.WithMetadata(auth.GatewayToGRPCMetadataAnnotator),
	)

	// We create a client connection that the Gateway AND the WebUI will use
	conn, err := grpc.NewClient(
		grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	if err := v1.RegisterQiVitalsServiceHandler(ctx, mux, conn); err != nil {
		return nil, nil, err
	}

	client := v1.NewQiVitalsServiceClient(conn)

	return mux, client, nil
}
