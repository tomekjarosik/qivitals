package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, textBody, htmlBody string) error
}

// App represents the composed gRPC + HTTP gateway + Web UI application.
type App struct {
	config        Config
	service       *QiVitalsService
	webHandler    http.Handler
	authenticator *auth.Authenticator
	emailSender   EmailSender
}

func NewApp(cfg Config, svc *QiVitalsService, webHandler http.Handler, authenticator *auth.Authenticator, emailSender EmailSender) *App {
	return &App{
		config:        cfg,
		service:       svc,
		webHandler:    webHandler,
		authenticator: authenticator,
		emailSender:   emailSender,
	}
}

// setupLogger now accepts the LogConfig struct and supports log levels.
func setupLogger(cfg LogConfig) (*slog.Logger, func(), error) {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		AddSource: level == slog.LevelDebug, // Only add source code location in debug mode
		Level:     level,
	}

	if cfg.File == "" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), func() {}, nil
	}

	logFile, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to open log file: %w", err)
	}

	cleanup := func() { logFile.Close() }
	return slog.New(slog.NewJSONHandler(logFile, opts)), cleanup, nil
}

func (a *App) Run(ctx context.Context) error {
	logger, cleanupLogger, err := setupLogger(a.config.Log)
	if err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	defer cleanupLogger()

	grpcServer := a.newGRPCServer(logger)
	forwardedHandler := middleware.ForwardedProtoMiddleware(a.webHandler)

	// Multiplex handler: Routing traffic over TLS based on HTTP/2 protocol and Content-Type
	mixedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			forwardedHandler.ServeHTTP(w, r)
		}
	})

	httpServer := &http.Server{
		Addr:    a.config.Server.Address,
		Handler: mixedHandler,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start unified Secure Server (HTTPS + gRPC)
	go func() {
		logger.Info("Secure unified server listening", slog.String("address", a.config.Server.Address))
		if err := httpServer.ListenAndServeTLS(a.config.TLS.CertFile, a.config.TLS.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
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
	interceptors = append(interceptors, middleware.LoggingInterceptor(logger), authInterceptor)
	var opts []grpc.ServerOption
	opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))

	grpcServer := grpc.NewServer(opts...)
	authorizedService := NewAuthorizedService(a.service, a.service)
	v1.RegisterQiVitalsServiceServer(grpcServer, authorizedService)

	magicLinkSvc := NewMagicLinkServer(a.config.MagicLink, a.authenticator, a.emailSender)
	v1.RegisterMagicLinkServiceServer(grpcServer, magicLinkSvc)

	return grpcServer
}

// NewGatewayHandler creates a gRPC-Gateway HTTP handler that forwards requests securely.
func NewGatewayHandler(ctx context.Context, grpcAddr string, certFile string) (http.Handler, v1.QiVitalsServiceClient, error) {
	mux := runtime.NewServeMux(
		runtime.WithMetadata(auth.GatewayToGRPCMetadataAnnotator),
	)
	runtime.WithForwardResponseOption(func(ctx context.Context, w http.ResponseWriter, m proto.Message) error {
		if resp, ok := m.(*v1.ValidateMagicLinkResponse); ok && resp.SessionToken != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     auth.SessionCookieName,
				Value:    resp.SessionToken,
				Path:     "/",
				HttpOnly: true,                 // XSS Protection
				Secure:   true,                 // HTTPS
				SameSite: http.SameSiteLaxMode, // CSRF Protection
				MaxAge:   86400,                // 24 hours
				Expires:  time.Now().Add(24 * time.Hour),
			})

			w.Header().Set("HX-Redirect", "/")
		}
		return nil
	})

	// Since the server is secure, the internal gateway client must trust the certificate
	creds, err := credentials.NewClientTLSFromFile(certFile, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load TLS credentials for gateway client: %w", err)
	}

	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, nil, err
	}

	if err := v1.RegisterQiVitalsServiceHandler(ctx, mux, conn); err != nil {
		return nil, nil, err
	}
	if err := v1.RegisterMagicLinkServiceHandler(ctx, mux, conn); err != nil {
		return nil, nil, err
	}

	client := v1.NewQiVitalsServiceClient(conn)

	return mux, client, nil
}
