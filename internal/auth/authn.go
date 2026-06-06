package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var SessionCookieDuration = 30 * 24 * time.Hour

// SSHKey holds the parsed crypto key and its fingerprint for quick lookup
type SSHKey struct {
	Fingerprint string           // Used as JWT "kid" (Key ID)
	CryptoKey   crypto.PublicKey // Used for signature verification
}

// UserRecord holds user-specific auth data
type UserRecord struct {
	Keys       map[string]*SSHKey // Map fingerprint -> key for O(1) lookup
	Namespaces []string
}

// MagicLinkStore defines the contract for persisting issued magic links.
// In production, replace this with Redis or a relational DB.
type MagicLinkStore interface {
	// Record stores the linkID with its email and expiry.
	Record(linkID string, email string, expiresAt time.Time) error
	// Validate checks if the link exists, matches email, hasn't expired, and is unused.
	// Returns nil if valid, then invalidates the link.
	Validate(linkID string, email string) error
}

type Authenticator struct {
	users       map[string]*UserRecord
	emailToUser map[string]string // Maps email -> username for WebUI routing

	webPrivKey     ed25519.PrivateKey
	webPubKey      ed25519.PublicKey
	magicLinkStore MagicLinkStore
}

func NewAuthenticator(cfg *UsersConfig, linkStore MagicLinkStore) (*Authenticator, error) {
	users := make(map[string]*UserRecord)
	emailToUser := make(map[string]string)

	for username, userCfg := range cfg.Users {
		keysMap := make(map[string]*SSHKey)
		for _, keyStr := range userCfg.PublicKeys {
			sshKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
			if err != nil {
				log.Printf("unable to parse key for user %s: %v", username, err)
				continue
			}

			cryptoKey, ok := sshKey.(ssh.CryptoPublicKey)
			if !ok {
				log.Printf("parsed key for user %s is not a crypto public key", username)
				continue
			}

			fingerprint := ssh.FingerprintSHA256(sshKey)
			keysMap[fingerprint] = &SSHKey{
				Fingerprint: fingerprint,
				CryptoKey:   cryptoKey.CryptoPublicKey(),
			}
		}

		users[username] = &UserRecord{
			Keys:       keysMap,
			Namespaces: userCfg.Namespaces,
		}

		for _, email := range userCfg.Emails {
			if existing, conflict := emailToUser[email]; conflict {
				log.Printf("warning: email %s is configured for both %s and %s", email, existing, username)
			}
			emailToUser[email] = username
		}
		log.Printf("configured user: %s | emails: %s | namespaces: %s",
			username,
			strings.Join(userCfg.Emails, ", "),
			strings.Join(userCfg.Namespaces, ", "))
	}

	// Generate secure Ed25519 keypair for Web/Magic Link flows
	webPub, webPriv, err := ed25519.GenerateKey(rand.Reader)

	// If WebKey path is configured, load from file; otherwise generate ephemeral
	if cfg.WebKey != "" {
		webPriv, webPub, err = LoadKeyPair(cfg.WebKey)
		if err != nil {
			return nil, fmt.Errorf("load web key from %s: %w", cfg.WebKey, err)
		}
	}

	return &Authenticator{
		users:          users,
		emailToUser:    emailToUser,
		webPrivKey:     webPriv,
		webPubKey:      webPub,
		magicLinkStore: linkStore,
	}, nil
}

func (a *Authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		return ctx, nil
	}

	tokenStr, found := strings.CutPrefix(authHeader[0], "Bearer ")
	if !found {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	entity, err := a.verifyToken(tokenStr)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "auth failed: %v", err)
	}

	return NewContextWithEntity(ctx, entity), nil
}

func (a *Authenticator) verifyToken(tokenStr string) (Entity, error) {
	var claims jwt.RegisteredClaims

	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		if claims.Subject == "" {
			return nil, fmt.Errorf("missing 'sub' claim")
		}

		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("missing 'kid' in token header")
		}

		if kid == "web-key-v1" {
			if claims.Issuer != "webui-session" {
				return nil, fmt.Errorf("invalid issuer for web token")
			}
			// Ensure the email (Subject) belongs to a known configured user
			if _, exists := a.emailToUser[claims.Subject]; !exists {
				return nil, fmt.Errorf("unknown email for web session")
			}
			return a.webPubKey, nil
		}

		user, exists := a.users[claims.Subject]
		if !exists {
			return nil, fmt.Errorf("unknown user")
		}

		sshKey, keyExists := user.Keys[kid]
		if !keyExists {
			return nil, fmt.Errorf("unknown or revoked key for user")
		}

		return sshKey.CryptoKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token validation failed")
	}

	kid, _ := token.Header["kid"].(string)
	var namespaces []string
	tokenType := "cli"
	entityID := claims.Subject // Default to subject (username for CLI)

	if kid == "web-key-v1" {
		tokenType = "web"
		username := a.emailToUser[claims.Subject] // Map email -> username
		entityID = username                       // Entity ID becomes the internal username
		namespaces = a.users[username].Namespaces
	} else {
		namespaces = a.users[claims.Subject].Namespaces
	}

	return &User{
		ID:                entityID,
		AllowedNamespaces: namespaces,
		Type:              tokenType,
	}, nil
}

// EntityFromRequest is used for WebUI
func (a *Authenticator) EntityFromRequest(r *http.Request) (Entity, error) {
	tokenHeader := GetTokenFromRequest(r)
	if tokenHeader == "" {
		return nil, fmt.Errorf("no authorization token found")
	}

	tokenStr, found := strings.CutPrefix(tokenHeader, BearerPrefix)
	if !found {
		return nil, fmt.Errorf("invalid authorization format")
	}

	return a.verifyToken(tokenStr)
}

// SessionClaims represents a long-lived web session
type SessionClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// GenerateMagicLink creates the JWT AND records it in the MagicLinkStore
func (a *Authenticator) GenerateMagicLink(email string) (string, error) {
	linkIDBytes := make([]byte, 32)
	if _, err := rand.Read(linkIDBytes); err != nil {
		return "", fmt.Errorf("generate link id: %w", err)
	}
	linkID := hex.EncodeToString(linkIDBytes)

	now := time.Now()
	expiresAt := now.Add(15 * time.Minute) // Short-lived magic link

	claims := MagicLinkClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "webui",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		LinkID: linkID,
		Email:  email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "web-key-v1"

	tokenStr, err := token.SignedString(a.webPrivKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	// Record in the store (enforces expiry and single-use later)
	if a.magicLinkStore != nil {
		if err := a.magicLinkStore.Record(linkID, email, expiresAt); err != nil {
			return "", fmt.Errorf("store magic link: %w", err)
		}
	}

	return tokenStr, nil
}

// IssueSessionToken generates a standard, longer-lived web session token
func (a *Authenticator) IssueSessionToken(email string) (string, error) {
	now := time.Now()
	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "webui-session",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(SessionCookieDuration)),
		},
		Email: email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "web-key-v1"

	return token.SignedString(a.webPrivKey)
}

// ParseAndValidateMagicLink verifies the JWT signature and enforces single-use.
func (a *Authenticator) ParseAndValidateMagicLink(tokenStr string) (*MagicLinkClaims, error) {
	claims := &MagicLinkClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		// Return the single web public key
		return a.webPubKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid magic link token: %w", err)
	}

	// Enforce one-time use & expiry in store
	if err := a.magicLinkStore.Validate(claims.LinkID, claims.Email); err != nil {
		return nil, fmt.Errorf("magic link validation failed: %w", err)
	}

	return claims, nil
}

func (a *Authenticator) IsEmailKnown(email string) bool {
	_, exists := a.emailToUser[email]
	return exists
}
