package auth

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// DefaultTokenExpiry is 7 days.
	DefaultTokenExpiry = 7 * 24 * time.Hour
)

// GenerateJWT creates a signed JWT for a user.
func GenerateJWT(privateKey ed25519.PrivateKey, username string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": username,
		"iat": now.Unix(),
		"exp": now.Add(DefaultTokenExpiry).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenStr, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenStr, nil
}
