package auth

// UserConfig holds the YAML config for a single user.
type UserConfig struct {
	PublicKeys []string `yaml:"public_keys" mapstructure:"public_keys"`
	Namespaces []string `yaml:"namespaces" mapstructure:"namespaces"`
	Emails     []string `yaml:"emails" mapstructure:"emails"`
}

// UsersConfig holds the top-level auth config.
type UsersConfig struct {
	Users  map[string]UserConfig `yaml:"users" mapstructure:"users"`
	WebKey string                `yaml:"web_key" mapstructure:"web_key"`
}
