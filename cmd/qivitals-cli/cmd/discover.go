package cmd

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/tomekjarosik/qivitals/internal/auth"
)

// DiscoverKey loads the identity private key from the config identity.path value.
//
// Config example:
//
//	identity:
//	  path: ~/.qivitals/key
func DiscoverKey() (ed25519.PrivateKey, error) {
	keyPath := viper.GetString("identity.path")
	if keyPath == "" {
		return nil, fmt.Errorf("identity.path not set in config — run 'qivitals-cli generate-keys' and add the path to your config")
	}

	// Expand ~ if present.
	if strings.HasPrefix(keyPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[1:])
	}

	privKey, _, err := auth.LoadKeyPair(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load identity key %s: %w", keyPath, err)
	}

	return privKey, nil
}

// GetJWTFunc creates a function that generates JWTs for the given username using the discovered key.
func GetJWTFunc(username string) func() (string, error) {
	return func() (string, error) {
		privKey, err := DiscoverKey()
		if err != nil {
			return "", err
		}
		return auth.GenerateJWT(privKey, username)
	}
}
