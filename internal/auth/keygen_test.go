package auth

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair failed: %v", err)
	}

	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("invalid private key size: %d bytes (expected %d)", len(priv), ed25519.PrivateKeySize)
	}
}

func TestSaveAndLoadKeyPair(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()

	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "test.key")
	pubPath := privPath + ".pub"

	err := SaveKeyPair(priv, privPath, "test@example.com")
	if err != nil {
		t.Fatalf("save key pair failed: %v", err)
	}

	// Check private key permissions.
	info, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("stat private key failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("invalid private key permissions: %v (expected 0600)", info.Mode().Perm())
	}

	// Check public key permissions.
	info, err = os.Stat(pubPath)
	if err != nil {
		t.Fatalf("stat public key failed: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("invalid public key permissions: %v (expected 0644)", info.Mode().Perm())
	}

	// Load key pair.
	loadedPriv, loadedPub, err := LoadKeyPair(privPath)
	if err != nil {
		t.Fatalf("load key pair failed: %v", err)
	}

	if !bytes.Equal(priv, loadedPriv) {
		t.Fatalf("private key mismatch")
	}
	if !bytes.Equal(pub, loadedPub) {
		t.Fatalf("public key mismatch")
	}
}
