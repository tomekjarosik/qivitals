package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tomekjarosik/one-status/internal/database"
	"github.com/tomekjarosik/one-status/internal/server"
	"github.com/tomekjarosik/one-status/internal/storage"
	"github.com/tomekjarosik/one-status/internal/web"
)

func NewCmdServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Starts the gRPC and HTTP gateway servers",
		Long:  "Launches the status service monitoring system with gRPC and an HTTP gateway.",
		RunE:  runServe,
	}

	// Flags – mapstructure tags in server.Config match these flag names
	cmd.Flags().String("grpc-port", "localhost:50051", "Address and port for gRPC server")
	cmd.Flags().String("http-port", "localhost:8088", "Address and port for HTTP gateway and UI")
	cmd.Flags().String("log-file", "statussvc.log", "Path to log file")
	cmd.Flags().String("database-url", "", "PostgreSQL connection URL (empty = in-memory storage)")
	cmd.Flags().Int32("database-max-conns", 10, "Maximum database connections")

	err := viper.BindPFlag("grpc_port", cmd.Flags().Lookup("grpc-port"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("http_port", cmd.Flags().Lookup("http-port"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("log_file", cmd.Flags().Lookup("log-file"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("database_url", cmd.Flags().Lookup("database-url"))
	if err != nil {
		return nil
	}
	err = viper.BindPFlag("database_max_conns", cmd.Flags().Lookup("database-max-conns"))
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
	if flagLocalDebug {
		fmt.Printf("Server Config: %+v\n", cfg)
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	var store storage.SensorStorage
	if cfg.DatabaseURL == "" {
		log.Println("WARNING: Using in-memory storage with naive periodic persistence")
		store = storage.NewSnapshotStorage(storage.NewMemorySensorStorage(), "onstatus.data", 5*time.Second)
	} else {
		dbPool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL, cfg.MaxConns)
		if err != nil {
			return err
		}
		defer dbPool.Close()
		store = storage.NewPostgresSensorStorage(dbPool)
	}

	statusSvc := server.NewStatusMonitorService(store)

	dashboard := web.NewDashboardHandler(statusSvc)
	details := web.NewSensorDetailsHandler(statusSvc)
	// TODO: Web should be single component, not 2: dashboard and details
	app := server.NewApp(cfg, statusSvc, dashboard, details)
	return app.Run(ctx)
}
