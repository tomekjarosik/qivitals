package server

import "github.com/tomekjarosik/qivitals/internal/auth"

type Config struct {
	DatabaseURL string `mapstructure:"database_url"`
	MaxConns    int32  `mapstructure:"database_max_conns"`
	GRPCPort    string `mapstructure:"grpc_port"`
	HTTPPort    string `mapstructure:"http_port"`
	LogFile     string `mapstructure:"log_file"`
	Verbose     bool   `mapstructure:"verbose"`

	Auth *auth.UsersConfig `mapstructure:"auth"`
}
