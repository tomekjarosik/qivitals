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
	var keyFile string

	var generateKeysCmd = &cobra.Command{
		Use:   "generate-keys",
		Short: "Generate an Ed25519 key pair",
		Long: `Generate an Ed25519 key pair, similar to ssh-keygen -t ed25519.

With -f/--file you specify the private key file path (e.g. -f ~/.ssh/mykey).
The public key is saved as <file>.pub.

Without -f, the tool looks for identity.path in config, and finally defaults to ~/.qivitals/key.

After generation, add the path to your config:

    identity:
      path: ~/.ssh/mykey   (or whatever you used with -f)

Add the public key (printed above) to /etc/qivitals/qivitals.yaml`,
		Run: func(cmd *cobra.Command, args []string) {
			privKey, _, err := auth.GenerateKeyPair()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to generate key pair: %v\n", err)
				os.Exit(1)
			}

			var privPath string
			// Determine private key path: flag > config > default
			if keyFile != "" {
				privPath = keyFile
				// Ensure the directory exists
				dir := filepath.Dir(privPath)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to create directory %s: %v\n", dir, err)
					os.Exit(1)
				}
			} else {
				// Fallback to config or default
				cfgPath := viper.GetString("identity.path")
				if cfgPath != "" {
					privPath = cfgPath
				} else {
					home, err := os.UserHomeDir()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: failed to get home directory: %v\n", err)
						os.Exit(1)
					}
					privPath = filepath.Join(home, ".qivitals", "key")
				}
				// Ensure directory exists
				dir := filepath.Dir(privPath)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to create directory %s: %v\n", dir, err)
					os.Exit(1)
				}
			}

			pubPath := privPath + ".pub"

			// Check for overwrite
			if _, err := os.Stat(privPath); err == nil {
				fmt.Fprintf(os.Stderr, "Error: private key file %s already exists\n", privPath)
				fmt.Fprintln(os.Stderr, "Remove it or use a different path with -f.")
				os.Exit(1)
			}
			if _, err := os.Stat(pubPath); err == nil {
				fmt.Fprintf(os.Stderr, "Error: public key file %s already exists\n", pubPath)
				fmt.Fprintln(os.Stderr, "Remove it or use a different path with -f.")
				os.Exit(1)
			}

			// Save keys
			if err := auth.SaveKeyPair(privKey, privPath, "qivitals-cli"); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save key pair: %v\n", err)
				os.Exit(1)
			}

			// Print public key
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

	// Use -f to match ssh-keygen's flag for output file
	generateKeysCmd.Flags().StringVarP(&keyFile, "file", "f", "", "Private key file path (like ssh-keygen -f)")

	return generateKeysCmd
}
