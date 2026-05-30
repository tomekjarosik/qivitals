package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
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

	auth, err := NewAuthenticator(cfg, NewMagicLinkStore())
	require.NoError(t, err)
	return auth
}

func TestAuthenticate(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	token, err := GenerateJWT(privKey, "user1")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	ctx, err = auth.Authenticate(ctx)
	require.NoError(t, err)

	entity := EntityFromContext(ctx)
	require.NotNil(t, entity, "expected entity")
	require.Equal(t, "user1", entity.SubjectID())
	require.Len(t, entity.Namespaces(), 2)
}

func TestAuthenticate_NoHeader(t *testing.T) {
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": mustGenKey()})

	ctx := context.Background()
	ctx, err := auth.Authenticate(ctx)
	require.NoError(t, err)
	require.Nil(t, EntityFromContext(ctx), "expected nil entity for unauthenticated request")
}

func TestAuthenticate_WrongKey(t *testing.T) {
	priv1, _, _ := GenerateKeyPair()
	priv2, _, _ := GenerateKeyPair()

	// User1 is configured with priv1
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": priv1})

	// But we sign the token with priv2
	token, err := GenerateJWT(priv2, "user1")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	_, err = auth.Authenticate(ctx)
	require.Error(t, err, "expected error for wrong key")
}

func TestAuthenticate_Expired(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	// We must manually calculate the kid so the server finds the key BEFORE checking expiry
	pub := privKey.Public().(ed25519.PublicKey)
	sshPub, _ := ssh.NewPublicKey(pub)
	kid := ssh.FingerprintSHA256(sshPub)

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   "user1",
		IssuedAt:  jwt.NewNumericDate(now.Add(-8 * 24 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(now.Add(-1 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid

	tokenStr, err := token.SignedString(privKey)
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenStr))
	_, err = auth.Authenticate(ctx)
	require.Error(t, err, "expected error for expired token")
}

func TestAuthenticate_UnknownUser(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"user1": privKey})

	token, err := GenerateJWT(privKey, "unknown_user")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	_, err = auth.Authenticate(ctx)
	require.Error(t, err, "expected error for unknown user")
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
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"testuser": priv})
	interceptor := ServerInterceptor(auth)

	token, err := GenerateJWT(priv, "testuser")
	require.NoError(t, err)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, nil, handler)
	require.NoError(t, err)

	entity := EntityFromContext(capturedCtx)
	require.NotNil(t, entity, "expected entity in context")
	require.Equal(t, "testuser", entity.SubjectID())
}

func TestAuthInterceptor_NoAuth(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"testuser": priv})
	interceptor := ServerInterceptor(auth)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err := interceptor(ctx, nil, nil, handler)
	require.NoError(t, err)

	require.Nil(t, EntityFromContext(capturedCtx), "expected nil entity for unauthenticated request")
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	priv, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"testuser": priv})
	interceptor := ServerInterceptor(auth)

	md := metadata.Pairs("authorization", "Bearer invalid.token.here")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	})
	require.Error(t, err, "expected error for invalid token")
}

func TestAuthInterceptor_WrongKey(t *testing.T) {
	priv1, _, _ := GenerateKeyPair()
	priv2, _, _ := GenerateKeyPair()
	auth := setupAuth(t, map[string]ed25519.PrivateKey{"testuser": priv1})
	interceptor := ServerInterceptor(auth)

	token, err := GenerateJWT(priv2, "testuser")
	require.NoError(t, err)

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	})
	require.Error(t, err, "expected error for wrong key")
}
func TestMagicLink_FullFlow_And_SingleUse(t *testing.T) {
	// setupAuth already initializes NewMagicLinkStore() internally
	auth := setupAuth(t, map[string]ed25519.PrivateKey{})

	email := "user@example.com"

	// Generate Magic Link (This will record it in your InMemoryMagicLinkStore)
	tokenStr, err := auth.GenerateMagicLink(email)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	// Validate Magic Link (First time - should succeed and mark as used in the store)
	claims, err := auth.ParseAndValidateMagicLink(tokenStr)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, email, claims.Email)
	require.Equal(t, email, claims.Subject)

	// Validate Magic Link (Second time - should fail due to single-use enforcement in the store)
	_, err = auth.ParseAndValidateMagicLink(tokenStr)
	require.Error(t, err, "expected error for reusing magic link")
	require.Contains(t, err.Error(), "magic link validation failed")
}

func TestMagicLink_ExpiredToken(t *testing.T) {
	auth := setupAuth(t, map[string]ed25519.PrivateKey{})
	email := "user@example.com"

	// Create an already expired token manually
	now := time.Now()
	claims := MagicLinkClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "webui",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired 1 hour ago
		},
		LinkID: "expired-link-id",
		Email:  email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "web-key-v1"

	tokenStr, err := token.SignedString(auth.webPrivKey)
	require.NoError(t, err)

	// jwt-go should reject this before it even hits the InMemoryMagicLinkStore
	_, err = auth.ParseAndValidateMagicLink(tokenStr)
	require.Error(t, err, "expected error for expired token")
	require.Contains(t, err.Error(), "invalid magic link token")
}

func TestMagicLink_TamperedPayload(t *testing.T) {
	auth := setupAuth(t, map[string]ed25519.PrivateKey{})

	// Generate a valid token
	tokenStr, err := auth.GenerateMagicLink("user@example.com")
	require.NoError(t, err)

	// Tamper with the token payload (change email without re-signing)
	parts := strings.Split(tokenStr, ".")
	require.Len(t, parts, 3)

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var payload map[string]interface{}
	err = json.Unmarshal(payloadBytes, &payload)
	require.NoError(t, err)

	payload["email"] = "hacker@example.com" // Tamper!

	newPayloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	parts[1] = base64.RawURLEncoding.EncodeToString(newPayloadBytes)
	tamperedToken := strings.Join(parts, ".")

	// Signature verification should catch the tampering
	_, err = auth.ParseAndValidateMagicLink(tamperedToken)
	require.Error(t, err, "expected error for tampered token")
	require.Contains(t, err.Error(), "invalid magic link token")
}

func TestIssueSessionToken(t *testing.T) {
	auth := setupAuth(t, map[string]ed25519.PrivateKey{})

	email := "user@example.com"
	tokenStr, err := auth.IssueSessionToken(email)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	// Parse and verify the session token using the Authenticator's public key
	claims := &SessionClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return auth.webPubKey, nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	require.Equal(t, email, claims.Email)
	require.Equal(t, email, claims.Subject)
	require.Equal(t, "webui-session", claims.Issuer)

	// Verify it has a longer expiry than the magic link (e.g., 24 hours)
	require.NotNil(t, claims.ExpiresAt)
	require.Greater(t, claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time), 23*time.Hour)
}

func TestVerifyToken_WebSession_MapsToUser(t *testing.T) {
	privKey, _, _ := GenerateKeyPair()
	pubKey := privKey.Public().(ed25519.PublicKey)
	sshPub, _ := ssh.NewPublicKey(pubKey)
	sshPubKeyStr := string(ssh.MarshalAuthorizedKey(sshPub))

	cfg := &UsersConfig{
		Users: map[string]UserConfig{
			"alice": {
				PublicKeys: []string{sshPubKeyStr},
				Namespaces: []string{"prod", "dev"},
				Emails:     []string{"alice@example.com", "alice@work.com"},
			},
		},
	}

	auth, err := NewAuthenticator(cfg, NewMagicLinkStore())
	require.NoError(t, err)

	tokenStr, err := auth.IssueSessionToken("alice@example.com")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenStr))
	ctx, err = auth.Authenticate(ctx)
	require.NoError(t, err)

	// Verify the Entity maps back to "alice" with her namespaces
	entity := EntityFromContext(ctx)
	require.NotNil(t, entity)

	require.Equal(t, "alice", entity.SubjectID(), "SubjectID should be the username, not the email")
	require.Equal(t, "web", entity.TokenType())
	require.ElementsMatch(t, []string{"prod", "dev"}, entity.Namespaces())
}

func TestVerifyToken_UnknownEmail(t *testing.T) {
	// Setup an authenticator with NO configured emails
	auth := setupAuth(t, map[string]ed25519.PrivateKey{})

	// Try to validate a token for an unknown email
	tokenStr, err := auth.IssueSessionToken("hacker@example.com")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenStr))
	_, err = auth.Authenticate(ctx)

	require.Error(t, err, "expected error for unknown email")
	require.Contains(t, err.Error(), "unknown email")
}
