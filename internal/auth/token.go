package auth

import (
	"crypto/ed25519"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"
)

const (
	// DefaultTokenExpiry is 7 days.
	DefaultTokenExpiry = 7 * 24 * time.Hour
)

// GenerateJWT creates a signed JWT for a user CLI.
// It automatically calculates the 'kid' (SSH fingerprint) from the private key.
func GenerateJWT(privateKey ed25519.PrivateKey, username string) (string, error) {
	pub := privateKey.Public().(ed25519.PublicKey)
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", err
	}

	kid := ssh.FingerprintSHA256(sshPub)

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   username,
		Issuer:    "cli",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid // Inject the fingerprint
	return token.SignedString(privateKey)
}
