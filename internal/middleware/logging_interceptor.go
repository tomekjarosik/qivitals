package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/tomekjarosik/qivitals/internal/canonicallog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		ctx = canonicallog.NewAccumulator(ctx)

		resp, err := handler(ctx, req)

		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		baseAttributes := []slog.Attr{
			slog.String("grpc.method", info.FullMethod),
			slog.Duration("duration", time.Since(start)),
			slog.String("grpc.status_code", statusCode.String()),
		}

		if err != nil {
			baseAttributes = append(baseAttributes, slog.String("error", err.Error()))
		}

		finalAttrs := baseAttributes
		if acc := canonicallog.GetAccumulator(ctx); acc != nil {
			finalAttrs = append(finalAttrs, acc.Fields()...)
		}

		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelError
		}

		logger.LogAttrs(ctx, level, "canonical_log", finalAttrs...)

		return resp, err
	}
}
