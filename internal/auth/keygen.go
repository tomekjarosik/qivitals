package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// GenerateKeyPair creates a standard Ed25519 key pair.
func GenerateKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key pair: %w", err)
	}
	return priv, pub, nil
}

// EncodePublicKey converts the key to the "ssh-ed25519 AAA..." format.
func EncodePublicKey(pub ed25519.PublicKey, comment string) ([]byte, error) {
	sshPubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, err
	}

	// MarshalAuthorizedKey adds the "ssh-ed25519 " prefix and base64 body.
	// We manually append the comment and newline.
	raw := ssh.MarshalAuthorizedKey(sshPubKey)
	out := fmt.Sprintf("%s %s\n", strings.TrimSpace(string(raw)), comment)
	return []byte(out), nil
}

// EncodePrivateKey converts the key to OpenSSH PEM format.
func EncodePrivateKey(priv ed25519.PrivateKey) ([]byte, error) {
	// ssh.MarshalPrivateKey returns a PEM block for the OpenSSH format.
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
}

// SaveKeyPair writes the keys to disk using standard SSH permissions.
func SaveKeyPair(priv ed25519.PrivateKey, path string, comment string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// Save Private Key (OpenSSH format)
	privEncoded, err := EncodePrivateKey(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, privEncoded, 0600); err != nil {
		return err
	}

	// Save Public Key (Authorized Key format)
	pubEncoded, err := EncodePublicKey(priv.Public().(ed25519.PublicKey), comment)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".pub", pubEncoded, 0644)
}

// LoadKeyPair reads keys from disk and returns standard crypto types.
func LoadKeyPair(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// ParseRawPrivateKey handles PEM decoding and type detection automatically.
	rawKey, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		return nil, nil, err
	}

	priv, ok := rawKey.(*ed25519.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("not an ed25519 key")
	}

	return *priv, priv.Public().(ed25519.PublicKey), nil
}
