package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// InitializeCommands sets up the root command and all subcommands.
func InitializeCommands() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "statussvc",
		Short: "One Status - a personal status page system for tracking life signals.",
		Long: `One Status is a system for tracking various personal and infrastructure signals
through push and pull mechanisms. It stores timestamps from diverse sources (backups,
payments, health checks) and displays them as a customizable status page.
Supports periodic check-ins via ed25519-signed signals, manual human inputs,
and automated endpoint monitoring, all stored in PostgreSQL and served via HTTP/gRPC.`,
		// NoArgs allows running just 'statussvc' to show help
		Args:                       cobra.NoArgs,
		SuggestionsMinimumDistance: 2,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := initConfig(); err != nil {
				return fmt.Errorf("failed to initialize config: %v", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(cmd.Short)
			return nil
		},
	}

	// Define the --verbose global flag
	var verbose bool
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Bind the verbose flag to Viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Add subcommands
	rootCmd.AddCommand(
		NewCmdServe(),
		// Add your other commands here
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

// initConfig handles configuration loading (Placeholder)
func initConfig() error {
	viper.AutomaticEnv()
	return nil
}
