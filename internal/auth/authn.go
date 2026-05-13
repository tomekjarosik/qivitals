package auth

import (
	"context"
	"crypto"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userStore struct {
	publicKey  crypto.PublicKey
	namespaces []string
}

type Authenticator struct {
	users map[string]userStore
}

func NewAuthenticator(cfg *UsersConfig) *Authenticator {
	store := make(map[string]userStore)

	for username, userCfg := range cfg.Users {
		if len(userCfg.PublicKeys) == 0 {
			continue
		}

		// Pre-parse the key at startup so we don't do it on every request
		sshKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(userCfg.PublicKeys[0]))
		if err != nil {
			continue // Or log a warning
		}

		if cryptoKey, ok := sshKey.(ssh.CryptoPublicKey); ok {
			store[username] = userStore{
				publicKey:  cryptoKey.CryptoPublicKey(),
				namespaces: userCfg.Namespaces,
			}
		}
	}

	return &Authenticator{users: store}
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
	var claims jwt.MapClaims

	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		// Ensure it's Ed25519
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		sub, _ := claims["sub"].(string)
		if u, ok := a.users[sub]; ok {
			return u.publicKey, nil
		}
		return nil, fmt.Errorf("unknown user")
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	sub, _ := claims["sub"].(string)
	return &User{
		ID:                sub,
		AllowedNamespaces: a.users[sub].namespaces,
	}, nil
}
