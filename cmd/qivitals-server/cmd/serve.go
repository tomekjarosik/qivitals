package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/database"
	"github.com/tomekjarosik/qivitals/internal/email"
	"github.com/tomekjarosik/qivitals/internal/server"
	"github.com/tomekjarosik/qivitals/internal/storage"
	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/handlers"
)

func NewCmdServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Starts the secure unified gRPC and HTTP gateway server",
		Long:  "Launches the status service monitoring system over a single secure TLS port.",
		RunE:  runServe,
	}

	// Consolidate into a single address flag and add TLS cert paths
	cmd.Flags().String("address", "localhost:8088", "Address and port for both gRPC and HTTP UI")
	cmd.Flags().String("log-file", "qivitals.log", "Path to log file")
	cmd.Flags().String("database-url", "", "PostgreSQL connection URL (empty = in-memory storage)")
	cmd.Flags().Int32("database-max-conns", 10, "Maximum database connections")
	cmd.Flags().String("tls-cert", "certs/server.crt", "Path to TLS certificate file")
	cmd.Flags().String("tls-key", "certs/server.key", "Path to TLS private key file")

	err := viper.BindPFlag("server.address", cmd.Flags().Lookup("address"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("server.log_file", cmd.Flags().Lookup("log-file"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("server.database_url", cmd.Flags().Lookup("database-url"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("server.database_max_conns", cmd.Flags().Lookup("database-max-conns"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("server.tls_cert_file", cmd.Flags().Lookup("tls-cert"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("server.tls_key_file", cmd.Flags().Lookup("tls-key"))
	if err != nil {
		return nil
	}

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	var cfg server.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	if viper.GetBool("verbose") {
		fmt.Printf("Server Config: %+v\n", cfg)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	var store storage.SensorStorage
	if cfg.Database.URL == "" {
		log.Println("WARNING: Using in-memory storage only")
		store = storage.NewMemorySensorStorage()
	} else if cfg.Database.URL == "naive-file" {
		log.Println("WARNING: Using in-memory storage with naive periodic persistence")
		store = storage.NewSnapshotStorage(storage.NewMemorySensorStorage(), "qivitals.data", 5*time.Second)
	} else {
		dbPool, err := database.NewPostgresPool(ctx, cfg.Database.URL, cfg.Database.MaxConns)
		if err != nil {
			return err
		}
		defer dbPool.Close()
		var pgStore = storage.NewPostgresSensorStorage(dbPool)
		ctx := context.Background()
		if err := pgStore.InitSchema(ctx); err != nil {
			log.Fatalf("Failed to initialize database schema: %v", err)
		}
		store = pgStore
	}

	qivitalsSvc := server.NewStatusMonitorService(store)
	renderer := web.NewTemplateRenderer()

	// Initialize individual handlers, passing the certificate file path for internal dialing
	gateway, grpcClient, err := server.NewGatewayHandler(ctx, cfg.Server.Address, cfg.TLS.CertFile)
	if err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}
	magicLinkStore := auth.NewMagicLinkStore()
	authenticator, err := auth.NewAuthenticator(cfg.Auth, magicLinkStore)

	dashboard := handlers.NewDashboardHandler(renderer, grpcClient, authenticator)
	details := handlers.NewSensorDetailsHandler(renderer, grpcClient)
	authHandler := handlers.NewWebAuthHandler(renderer, authenticator)

	webRouter := web.NewRouter(gateway, dashboard, details, authHandler)

	if err != nil {
		return fmt.Errorf("failed to initialize authenticator: %w", err)
	}
	var emailSender email.Sender
	switch strings.ToLower(cfg.Email.SenderType) {
	case "file":
		emailSender = email.NewFileEmailSender(cfg.Email.FilePath)
	default:
		emailSender = email.NewSMTPSender(
			cfg.Email.FromEmail,
			cfg.Email.HostPort,
			cfg.Email.Username,
			cfg.Email.Password)
	}

	app := server.NewApp(cfg, qivitalsSvc, webRouter, authenticator, emailSender)
	return app.Run(ctx)
}
