package middleware

import (
	"net/http"

	"google.golang.org/grpc/metadata"
)

func GetTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie("session_token"); err == nil {
		return "Bearer " + cookie.Value
	}
	return r.Header.Get("Authorization")
}

// InjectMetadata wraps an http.Handler and injects the Auth header into the gRPC context
func InjectMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		token = GetTokenFromRequest(r)

		// Inject into gRPC outgoing context
		// We use the standard "authorization" key so the gRPC interceptor
		// doesn't have to care where the token originally came from.
		ctx := metadata.AppendToOutgoingContext(r.Context(), "authorization", token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
