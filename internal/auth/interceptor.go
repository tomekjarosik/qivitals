package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

// ClientInterceptor creates a gRPC interceptor that injects the JWT as a Bearer token.
func ClientInterceptor(jwtToken string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if jwtToken != "" {
			md := metadata.Pairs("authorization", "Bearer "+jwtToken)
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
