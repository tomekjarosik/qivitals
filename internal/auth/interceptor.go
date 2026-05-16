package auth

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// SessionCookieName is the name of the cookie containing the JWT.
	SessionCookieName = "session_token"

	// AuthHeaderKey is the HTTP/gRPC metadata key for authorization.
	// Note: gRPC metadata keys are case-insensitive, but lowercase is standard.
	AuthHeaderKey = "authorization"

	// BearerPrefix is the standard prefix for JWT tokens in HTTP Authorization headers.
	BearerPrefix = "Bearer "
)

// ServerInterceptor is a gRPC unary server interceptor that performs authentication.
func ServerInterceptor(auth *Authenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Authenticate (may return unauthenticated context).
		ctx, err := auth.Authenticate(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// JWTClientInterceptor creates a gRPC interceptor that injects the JWT as a Bearer token.
func JWTClientInterceptor(jwtToken string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if jwtToken != "" {
			// Construct the "Bearer <token>" value using the constant prefix.
			md := metadata.Pairs(AuthHeaderKey, BearerPrefix+jwtToken)
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// GetTokenFromRequest extracts the token from a cookie or header and formats it for gRPC.
func GetTokenFromRequest(r *http.Request) string {
	// If cookie exists, format it as "Bearer <value>".
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return BearerPrefix + cookie.Value
	}
	// Otherwise, return whatever is in the header (or empty string).
	return r.Header.Get(AuthHeaderKey)
}

// InjectAuthContext wraps an http.Handler and injects the Auth header into the gRPC context.
func InjectAuthContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetTokenFromRequest(r)
		if token != "" {
			// Append the "authorization" metadata to the request context.
			ctx := metadata.AppendToOutgoingContext(r.Context(), AuthHeaderKey, token)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// GatewayToGRPCMetadataAnnotator extracts auth info from the HTTP request
// and returns it as gRPC metadata for the gateway.
func GatewayToGRPCMetadataAnnotator(ctx context.Context, r *http.Request) metadata.MD {
	token := GetTokenFromRequest(r)
	if token == "" {
		return make(metadata.MD)
	}
	return metadata.Pairs(AuthHeaderKey, token)
}
