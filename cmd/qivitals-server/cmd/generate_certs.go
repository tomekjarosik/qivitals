package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func NewCmdGenerateCerts() *cobra.Command {
	var outputDir string
	var commonName string
	var validityDays int

	cmd := &cobra.Command{
		Use:   "generate-certs",
		Short: "Generate self-signed TLS certificates for local development",
		Long: `Generates a self-signed ECDSA P-256 private key and X.509 certificate for local TLS development.
This is more efficient and secure than RSA for TLS connections.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputDir == "" {
				outputDir = "certs"
			}

			if commonName == "" {
				commonName = "localhost"
			}

			// Create directory if it doesn't exist
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
			}

			certPath := filepath.Join(outputDir, "server.crt")
			keyPath := filepath.Join(outputDir, "server.key")

			fmt.Printf("Generating ECDSA P-256 private key...\n")
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return fmt.Errorf("failed to generate private key: %w", err)
			}

			// Create certificate template
			serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
			if err != nil {
				return fmt.Errorf("failed to generate serial number: %w", err)
			}

			template := x509.Certificate{
				SerialNumber: serial,
				Subject: pkix.Name{
					Organization: []string{"QiVitals Dev"},
					CommonName:   commonName,
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().AddDate(0, 0, validityDays),
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
			}

			// Add Subject Alternative Names (SANs) for localhost
			template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
			template.IPAddresses = append(template.IPAddresses, net.ParseIP("::1"))
			template.DNSNames = append(template.DNSNames, "localhost")

			// Self-sign the certificate
			certDERBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
			if err != nil {
				return fmt.Errorf("failed to create certificate: %w", err)
			}

			// Write Private Key (PKCS8 format for ECDSA)
			keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return fmt.Errorf("failed to marshal private key: %w", err)
			}
			keyFile, err := os.Create(keyPath)
			if err != nil {
				return fmt.Errorf("failed to create key file: %w", err)
			}
			keyPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
			if _, err := keyFile.Write(keyPem); err != nil {
				keyFile.Close()
				return err
			}
			keyFile.Close()

			// Write Certificate
			certFile, err := os.Create(certPath)
			if err != nil {
				return fmt.Errorf("failed to create cert file: %w", err)
			}
			certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDERBytes})
			if _, err := certFile.Write(certPem); err != nil {
				certFile.Close()
				return err
			}
			certFile.Close()

			fmt.Printf("Certificates generated successfully:\n")
			fmt.Printf("  - Private Key (ECDSA P-256): %s\n", keyPath)
			fmt.Printf("  - Certificate: %s\n", certPath)
			fmt.Printf("\nTo use these certificates, update your config.yaml:\n")
			fmt.Printf("server:\n")
			fmt.Printf("  tls_cert_file: %s\n", certPath)
			fmt.Printf("  tls_key_file: %s\n", keyPath)

			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "certs", "Directory to save certificates")
	cmd.Flags().StringVarP(&commonName, "common-name", "", "localhost", "Common Name (CN) for the certificate")
	cmd.Flags().IntVar(&validityDays, "days", 365, "Certificate validity in days")

	return cmd
}
