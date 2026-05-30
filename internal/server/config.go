package server

import (
	"fmt"

	"github.com/tomekjarosik/qivitals/internal/auth"
)

// Config holds the configuration for the server.
// Config is the root configuration structure.
type Config struct {
	Server    ServerConfig      `mapstructure:"server"`
	Database  DatabaseConfig    `mapstructure:"database"`
	Log       LogConfig         `mapstructure:"log"`
	TLS       TLSConfig         `mapstructure:"tls"`
	Auth      *auth.UsersConfig `mapstructure:"auth"`
	MagicLink MagicLinkConfig   `mapstructure:"magic_link"`
	Email     EmailConfig       `mapstructure:"email"`
}

type EmailConfig struct {
	SenderType string `mapstructure:"sender_type"` // "smtp" or "file"
	FilePath   string `mapstructure:"file_path"`   // Used if sender_type is "file"
	FromEmail  string `mapstructure:"from_email"`
	HostPort   string `mapstructure:"host_port"` // e.g., "email-smtp.eu-central-1.amazonaws.com:587"
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
}
type ServerConfig struct {
	Address string `mapstructure:"address"` // e.g., ":8080" or "0.0.0.0:443"
}

type DatabaseConfig struct {
	URL      string `mapstructure:"url"`
	MaxConns int32  `mapstructure:"max_conns"`
}

type LogConfig struct {
	File  string `mapstructure:"file"`  // Empty string means stdout
	Level string `mapstructure:"level"` // debug, info, warn, error
}

type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type MagicLinkConfig struct {
	AppBaseURL string `mapstructure:"app_base_url"` // e.g., "https://app.qivitals.com"
	AppName    string `mapstructure:"app_name"`
	FromEmail  string `mapstructure:"from_email"`
}

// Validate checks if the configuration is valid before starting the app.
func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
		return fmt.Errorf("tls.cert_file and tls.key_file are required when tls.enabled is true")
	}
	if c.MagicLink.AppBaseURL == "" {
		return fmt.Errorf("magic_link.app_base_url is required")
	}
	if c.MagicLink.FromEmail == "" {
		return fmt.Errorf("magic_link.from_email is required")
	}
	if c.Email.FromEmail == "" {
		return fmt.Errorf("email.from_email is required")
	}

	// Validation for "file" sender
	if c.Email.SenderType == "file" && c.Email.FilePath == "" {
		return fmt.Errorf("email.file_path is required when sender_type is 'file'")
	}

	// Validation for "smtp" sender
	if c.Email.SenderType == "smtp" {
		if c.Email.HostPort == "" {
			return fmt.Errorf("email.host_port is required when sender_type is 'smtp'")
		}
		if c.Email.Username == "" || c.Email.Password == "" {
			return fmt.Errorf("email.username and email.password are required when sender_type is 'smtp'")
		}
	}

	return nil
}
