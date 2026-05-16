package server

import "github.com/tomekjarosik/qivitals/internal/auth"

// Config holds the configuration for the server.
type Config struct {
	Address     string            `mapstructure:"address"`
	LogFile     string            `mapstructure:"log_file"`
	DatabaseURL string            `mapstructure:"database_url"`
	MaxConns    int32             `mapstructure:"database_max_conns"`
	TLSCertFile string            `mapstructure:"tls_cert_file"`
	TLSKeyFile  string            `mapstructure:"tls_key_file"`
	Auth        *auth.UsersConfig `mapstructure:"auth"`
}
