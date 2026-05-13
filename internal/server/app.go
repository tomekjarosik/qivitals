package server

import (
	"context"
	"errors"
	"log"
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
	"google.golang.org/grpc/metadata"
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
	grpcServer := a.newGRPCServer()
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
func (a *App) newGRPCServer() *grpc.Server {

	authInterceptor := auth.ServerInterceptor(a.authenticator)
	interceptors := make([]grpc.UnaryServerInterceptor, 0)
	if a.config.LogFile != "" {
		interceptors = append(interceptors, middleware.LoggingInterceptor(a.config.LogFile))
	}
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
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			token := middleware.GetTokenFromRequest(r)
			return metadata.Pairs("authorization", token)
		}),
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
