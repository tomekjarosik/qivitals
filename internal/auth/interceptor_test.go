package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestGetTokenFromRequest_CookiePrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})
	req.Header.Set(AuthHeaderKey, "Bearer header-token")

	got := GetTokenFromRequest(req)
	require.Equal(t, "Bearer cookie-token", got)
}

func TestGetTokenFromRequest_OnlyHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(AuthHeaderKey, "Bearer header-token")

	got := GetTokenFromRequest(req)
	require.Equal(t, "Bearer header-token", got)
}

func TestGetTokenFromRequest_OnlyCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})

	got := GetTokenFromRequest(req)
	require.Equal(t, "Bearer cookie-token", got)
}

func TestGetTokenFromRequest_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got := GetTokenFromRequest(req)
	require.Empty(t, got)
}

func TestInjectAuthContext_Cookie(t *testing.T) {
	var capturedCtx context.Context

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
	})

	wrappedHandler := InjectAuthContext(handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok, "expected outgoing context")
	require.NotNil(t, md)

	vals := md.Get(AuthHeaderKey)
	require.Len(t, vals, 1)
	require.Equal(t, "Bearer cookie-token", vals[0])
}

func TestInjectAuthContext_Header(t *testing.T) {
	var capturedCtx context.Context

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
	})

	wrappedHandler := InjectAuthContext(handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(AuthHeaderKey, "Bearer header-token")

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok, "expected outgoing context")
	require.NotNil(t, md)

	vals := md.Get(AuthHeaderKey)
	require.Len(t, vals, 1)
	require.Equal(t, "Bearer header-token", vals[0])
}

func TestInjectAuthContext_Empty(t *testing.T) {
	var capturedCtx context.Context

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
	})

	wrappedHandler := InjectAuthContext(handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	if ok {
		vals := md.Get(AuthHeaderKey)
		require.Empty(t, vals)
	}
}

func TestGatewayToGRPCMetadataAnnotator_Cookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})

	md := GatewayToGRPCMetadataAnnotator(context.Background(), req)
	require.NotNil(t, md)

	vals := md.Get(AuthHeaderKey)
	require.Len(t, vals, 1)
	require.Equal(t, "Bearer cookie-token", vals[0])
}

func TestGatewayToGRPCMetadataAnnotator_Header(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(AuthHeaderKey, "Bearer header-token")

	md := GatewayToGRPCMetadataAnnotator(context.Background(), req)
	require.NotNil(t, md)

	vals := md.Get(AuthHeaderKey)
	require.Len(t, vals, 1)
	require.Equal(t, "Bearer header-token", vals[0])
}

func TestGatewayToGRPCMetadataAnnotator_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	md := GatewayToGRPCMetadataAnnotator(context.Background(), req)

	// Expecting nil or empty map as per user requirement
	require.Empty(t, md)
}

func TestJWTClientInterceptor_WithToken(t *testing.T) {
	var capturedCtx context.Context

	mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx
		return nil
	}

	interceptor := JWTClientInterceptor("jwt-token")
	err := interceptor(context.Background(), "/mock/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok, "expected outgoing context")
	require.NotNil(t, md)

	vals := md.Get(AuthHeaderKey)
	require.Len(t, vals, 1)
	require.Equal(t, "Bearer jwt-token", vals[0])
}

func TestJWTClientInterceptor_EmptyToken(t *testing.T) {
	var capturedCtx context.Context

	mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx
		return nil
	}

	interceptor := JWTClientInterceptor("")
	err := interceptor(context.Background(), "/mock/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	// Verify that no auth metadata was added to the outgoing context
	md, ok := metadata.FromOutgoingContext(capturedCtx)
	if ok {
		vals := md.Get(AuthHeaderKey)
		require.Empty(t, vals)
	}
}
