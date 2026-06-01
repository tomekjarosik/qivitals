package cmd

import (
	"crypto/ed25519"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tomekjarosik/qivitals/internal/auth"
)

func NewCmdGenerateWebkey() *cobra.Command {
	var keyPath string
	var comment string

	cmd := &cobra.Command{
		Use:   "generate-webkey",
		Short: "Generate an Ed25519 keypair for web authentication",
		Long: `Generate a new Ed25519 keypair for web authentication and save to file.

The private key is saved as an OpenSSH PEM file and the public key is saved
as an authorized key file. These keys are used for signing JWT session tokens
in the web authentication flow.

By default, keys are saved to the configured auth directory (or "certs/" if
no config is available).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve output path
			if keyPath == "" {
				keyPath = viper.GetString("webkey.path")
			}
			if keyPath == "" {
				keyPath = "certs/webkey"
			}

			if comment == "" {
				comment = "qivitals-web-key"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Generating new Ed25519 keypair...\n")

			priv, pub, err := auth.GenerateKeyPair()
			if err != nil {
				return fmt.Errorf("generate keypair: %w", err)
			}

			err = auth.SaveKeyPair(priv, keyPath, comment)
			if err != nil {
				return fmt.Errorf("save keys: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Private key: %s\n", keyPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Public key:  %s.pub\n", keyPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Fingerprint: %s\n", keyFingerprint(pub))

			fmt.Fprintf(cmd.OutOrStdout(), "\nAdd to your config.yaml:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  auth:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "    web_key: %s\n", keyPath)

			return nil
		},
	}

	cmd.Flags().StringVar(&keyPath, "path", "", "Output path for key files (default: certs/webkey)")
	cmd.Flags().StringVar(&comment, "comment", "qivitals-web-key", "Comment to append to public key")

	_ = viper.BindPFlag("webkey.path", cmd.Flags().Lookup("path"))

	return cmd
}

// keyFingerprint returns a short identifier for the public key.
func keyFingerprint(pub ed25519.PublicKey) string {
	return fmt.Sprintf("%x", pub[:8])
}
