package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"
)

var flagConfigFile string
var flagLocalDebug bool

// initConfig handles configuration loading
func initConfig() error {
	if flagConfigFile != "" {
		viper.SetConfigFile(flagConfigFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		viper.AddConfigPath(path.Join(home, ".onestatus"))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Allow environment variables to override the config file
	// This allows: DATABASE_URL=... to override the file
	viper.AutomaticEnv()
	// This tells Viper: "When looking for an Environment Variable,
	// replace dots with underscores."
	// So, if the user sets GRPC_PORT, Viper maps it to "grpc.port" or "grpc-port"
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if flagLocalDebug {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
		fmt.Println(viper.AllSettings())
	}
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if the config file is missing, as long as env vars are present
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	return nil
}
