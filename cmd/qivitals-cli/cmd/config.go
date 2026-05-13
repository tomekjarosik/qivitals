package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"
)

var flagConfigFile string

// initConfig handles configuration loading
func initConfig() error {
	if flagConfigFile != "" {
		viper.SetConfigFile(flagConfigFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		viper.AddConfigPath(path.Join(home, ".qivitals"))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Allow environment variables to override the config file
	// This allows: DATABASE_URL=... to override the file
	viper.SetEnvPrefix("QIVITALS")
	viper.AutomaticEnv()

	// This tells Viper: "When looking for an Environment Variable,
	// replace dots with underscores."
	// So, if the user sets GRPC_PORT, Viper maps it to "grpc.port" or "grpc-port"
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if err := viper.ReadInConfig(); err != nil {
		// Print a clear message instead of silently swallowing.
		fmt.Fprintf(os.Stderr, "[config] using: %s\n", viper.ConfigFileUsed())
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintf(os.Stderr, "[config] no config file found at searched paths\n")
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Always print debug info when --local-debug is passed
	if viper.GetBool("verbose") {
		fmt.Println("Config loaded:", viper.ConfigFileUsed())
		fmt.Println("All settings:", viper.AllSettings())
	}

	return nil
}
