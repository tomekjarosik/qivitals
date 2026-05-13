package auth

// UserConfig holds the YAML config for a single user.
type UserConfig struct {
	PublicKeys []string `yaml:"publicKeys"`
	Namespaces []string `yaml:"namespaces"`
}

// UsersConfig holds the top-level auth config.
type UsersConfig struct {
	Users map[string]UserConfig `yaml:"users"`
}
