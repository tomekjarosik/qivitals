package auth

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/metadata"
)

// setupAuth creates an Authenticator with the given private keys.
func setupAuth(t *testing.T, privKeys map[string]ed25519.PrivateKey) *Authenticator {
	t.Helper()

	cfg := &UsersConfig{
		Users: make(map[string]UserConfig),
	}

	for username, privKey := range privKeys {
		pubKey := privKey.Public().(ed25519.PublicKey)

		// Wrap raw Ed25519 bytes into an ssh.PublicKey object
		sshPub, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			t.Fatalf("failed to create SSH public key for %s: %v", username, err)
		}

		// MarshalAuthorizedKey expects an ssh.PublicKey interface
		sshPubKey := ssh.MarshalAuthorizedKey(sshPub)

		cfg.Users[username] = UserConfig{
			PublicKeys: []string{string(sshPubKey)},
			Namespaces: []string{"home", "infra"},
		}
	}

	return NewAuthenticator(cfg)
}

func TestAuthenticate(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	token, err := GenerateJWT(privKey, "user1")
	if err != nil {
		t.Fatal(err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	ctx, err = auth.Authenticate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	entity := AuthEntityFromContext(ctx)
	if entity == nil {
		t.Fatal("expected entity")
	}
	if entity.SubjectID() != "user1" {
		t.Fatalf("expected user1, got %s", entity.SubjectID())
	}
	if len(entity.Namespaces()) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(entity.Namespaces()))
	}
}

func TestAuthenticate_NoHeader(t *testing.T) {
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": mustGenKey()})

	ctx := context.Background()
	ctx, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if AuthEntityFromContext(ctx) != nil {
		t.Fatal("expected nil entity for unauthenticated request")
	}
}

func TestAuthenticate_WrongKey(t *testing.T) {
	priv1, _, _ := GenerateKeyPair()
	priv2, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": priv1})

	token, _ := GenerateJWT(priv2, "user1")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestAuthenticate_Expired(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user1",
		"iat": now.Add(-8 * 24 * time.Hour).Unix(),
		"exp": now.Add(-1 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenStr, _ := token.SignedString(privKey)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenStr))
	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestAuthenticate_UnknownUser(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	token, _ := GenerateJWT(privKey, "unknown_user")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func mustGenKey() ed25519.PrivateKey {
	priv, _, _ := GenerateKeyPair()
	return priv
}

func newTestConfig(t *testing.T, privKeys map[string]ed25519.PrivateKey) *UsersConfig {
	t.Helper()

	cfg := &UsersConfig{
		Users: make(map[string]UserConfig),
	}

	for username, privKey := range privKeys {
		pubKey := privKey.Public().(ed25519.PublicKey)

		// Wrap raw Ed25519 bytes into an ssh.PublicKey object
		sshPub, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			t.Fatalf("failed to create SSH public key for %s: %v", username, err)
		}

		// MarshalAuthorizedKey expects an ssh.PublicKey interface
		sshPubKey := ssh.MarshalAuthorizedKey(sshPub)

		cfg.Users[username] = UserConfig{
			PublicKeys: []string{string(sshPubKey)},
			Namespaces: []string{"home", "infra"},
		}
	}

	return cfg
}

func TestAuthInterceptor(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	authCfg := newTestConfig(t, map[string]ed25519.PrivateKey{"testuser": priv})

	authenticator := NewAuthenticator(authCfg)
	interceptor := ServerInterceptor(authenticator)

	token, err := GenerateJWT(priv, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, nil, handler)
	if err != nil {
		t.Fatal(err)
	}

	entity := AuthEntityFromContext(capturedCtx)
	if entity == nil {
		t.Fatal("expected entity in context")
	}
	if entity.SubjectID() != "testuser" {
		t.Fatalf("expected testuser, got %s", entity.SubjectID())
	}
}

func TestAuthInterceptor_NoAuth(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	authCfg := newTestConfig(t, map[string]ed25519.PrivateKey{"testuser": priv})

	interceptor := ServerInterceptor(NewAuthenticator(authCfg))

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err := interceptor(ctx, nil, nil, handler)
	if err != nil {
		t.Fatal(err)
	}

	if AuthEntityFromContext(capturedCtx) != nil {
		t.Fatal("expected nil entity for unauthenticated request")
	}
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	authCfg := newTestConfig(t, map[string]ed25519.PrivateKey{"testuser": priv})

	interceptor := ServerInterceptor(NewAuthenticator(authCfg))

	md := metadata.Pairs("authorization", "Bearer invalid.token.here")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	})
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthInterceptor_WrongKey(t *testing.T) {
	priv1, _, _ := GenerateKeyPair()
	priv2, _, _ := GenerateKeyPair()
	authCfg := newTestConfig(t, map[string]ed25519.PrivateKey{"testuser": priv1})

	interceptor := ServerInterceptor(NewAuthenticator(authCfg))

	token, err := GenerateJWT(priv2, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	})
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}
