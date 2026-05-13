package cmd

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/tomekjarosik/qivitals/internal/auth"

	"github.com/spf13/cobra"
)

func NewGenerateKeysCmd() *cobra.Command {

	var generateKeysCmd = &cobra.Command{
		Use:   "generate-keys",
		Short: "Generate an Ed25519 key pair",
		Long: `Generate an Ed25519 key pair, similar to ssh-keygen -t ed25519.

The private key is saved to ~/.qivitals/key (mode 600) and the public key
to ~/.qivitals/key.pub (mode 644).

After generation, add the path to your config:

    identity:
      path: ~/.qivitals/key

Add the public key (printed above) to /etc/qivitals/qivitals.yaml`,
		Run: func(cmd *cobra.Command, args []string) {
			privKey, _, err := auth.GenerateKeyPair()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to generate key pair: %v\n", err)
				os.Exit(1)
			}

			// Determine output directory — respect config, then default.
			dir := viper.GetString("identity.path")
			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to get home directory: %v\n", err)
					os.Exit(1)
				}
				dir = filepath.Join(home, ".qivitals")
			} else if !filepath.IsAbs(dir) {
				// identity.path could be just a directory or a full path — normalize.
				if ext := filepath.Ext(dir); ext == ".key" || ext == ".pem" {
					dir = filepath.Dir(dir)
				}
			}

			privPath := filepath.Join(dir, "key")
			pubPath := privPath + ".pub"

			// Check for overwrite.
			if _, err := os.Stat(privPath); err == nil {
				fmt.Fprintf(os.Stderr, "Error: key file %s already exists\n", privPath)
				fmt.Fprintln(os.Stderr, "Remove it or specify a different identity.path in your config.")
				os.Exit(1)
			}

			if err := os.MkdirAll(dir, 0o700); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to create directory %s: %v\n", dir, err)
				os.Exit(1)
			}

			if err := auth.SaveKeyPair(privKey, privPath, "qivitals-cli"); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save key pair: %v\n", err)
				os.Exit(1)
			}

			pubBytes, err := auth.EncodePublicKey(privKey.Public().(ed25519.PublicKey), "qivitals-cli")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to encode public key: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(pubBytes))

			fmt.Fprintf(os.Stderr, "\nKey pair saved to %s and %s\n", privPath, pubPath)
			fmt.Fprintln(os.Stderr, "Add the private key path to your config:")
			fmt.Fprintf(os.Stderr, "  identity:\n    path: %s\n", privPath)
		},
	}
	return generateKeysCmd
}
