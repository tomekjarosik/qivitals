package middleware

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
)

// LoggingInterceptor returns a UnaryServerInterceptor that logs request and response as one-liners.
func LoggingInterceptor(logFilePath string) grpc.UnaryServerInterceptor {
	// Open the log file in append mode, create it if it doesn't exist
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// If we can't open the file, fallback to standard logger
		log.Fatalf("failed to open grpc log file: %v", err)
	}

	logger := log.New(f, "", log.LstdFlags)

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)

		// Format the one-liner
		logLine := fmt.Sprintf("method=%s req=%+v resp=%+v err=%v",
			info.FullMethod, req, resp, err)

		// Write to the file
		logger.Println(logLine)

		return resp, err
	}
}
