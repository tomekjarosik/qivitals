package cmd

import (
	"github.com/spf13/viper"
)

// initConfig handles configuration loading (Placeholder)
func initConfig() error {
	viper.SetDefault("url", "localhost:50051")
	// Add the prefix. Viper will automatically look for QIVITALS_URL
	viper.SetEnvPrefix("QIVITALS")
	viper.AutomaticEnv()
	return nil
}
