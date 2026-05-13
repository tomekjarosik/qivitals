package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewQiVitalsClient abstracts the boilerplate of gRPC connection and client creation.
func NewQiVitalsClient(ctx context.Context) (v1.QiVitalsServiceClient, *grpc.ClientConn, error) {
	target := viper.GetString("url")

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := v1.NewQiVitalsServiceClient(conn)
	return client, conn, nil
}

// InitializeCommands sets up the root command and all subcommands.
func InitializeCommands() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "qivitals-cli",
		Short: "QiVitals - a CLI for managing and monitoring status signals.",
		Long: `qivitals-cli is a command-line tool for interacting with the QiVitals service.
Register sensors, send health check signals, and query sensor statuses all from the terminal.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := initConfig(); err != nil {
				return fmt.Errorf("failed to initialize config: %v", err)
			}
			return nil
		},
	}

	// Define the --verbose global flag
	var verbose bool
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Define the --url global flag for the service endpoint
	var baseURL string
	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "localhost:50051", "QiVitals service gRPC endpoint (host:port)")
	viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))

	// Define the --machine global flag for JSON output
	var machineOutput bool
	rootCmd.PersistentFlags().BoolVarP(&machineOutput, "machine", "m", false, "output response in machine-readable JSON format")
	viper.BindPFlag("machine", rootCmd.PersistentFlags().Lookup("machine"))

	// Add subcommands
	rootCmd.AddCommand(
		NewCmdRegister(),
		NewCmdReport(),
		NewCmdQuery(),
		NewCmdStatus(),
		NewCmdDelete(),
		NewCmdUpdate(),
		NewGenerateKeysCmd(),
	)

	return rootCmd
}

// Execute runs the provided command and handles signals for graceful shutdown.
func Execute(rootCmd *cobra.Command) {
	// Use a context that listens for the interrupt signal (Ctrl+C)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// We wrap the execution in a function to handle the context
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
