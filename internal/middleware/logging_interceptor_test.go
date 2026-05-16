// ... existing code ...
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomekjarosik/qivitals/internal/canonicallog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// captureHandler intercepts slog records and stores them for inspection.
type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{records: h.records}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	return h
}

// findAttr iterates over a Record's Attrs and returns the first one matching the given key, or nil.
func findAttr(rec slog.Record, key string) *slog.Attr {
	var found *slog.Attr
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			found = &a
			return false
		}
		return true
	})
	return found
}

func TestLoggingInterceptor_Succeeds(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	expectedResp := "hello"
	expectedMethod := "/test.Service/Method"

	info := &grpc.UnaryServerInfo{
		FullMethod: expectedMethod,
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return expectedResp, nil
	}

	_, err := interceptor(context.Background(), nil, info, actualHandler)
	require.NoError(t, err)

	require.Len(t, handler.records, 1)
	rec := handler.records[0]

	assert.Equal(t, "canonical_log", rec.Message)
	assert.Equal(t, slog.LevelInfo, rec.Level)

	methodAttr := findAttr(rec, "grpc.method")
	require.NotNil(t, methodAttr, "'grpc.method' attribute should be present")
	assert.Equal(t, expectedMethod, methodAttr.Value.String(), "'method' value should match")

	statusAttr := findAttr(rec, "grpc.status_code")
	require.NotNil(t, statusAttr, "'grpc.status_code' attribute should be present")
	assert.Equal(t, "OK", statusAttr.Value.String(), "'grpc.status_code' should be OK")

	assert.Nil(t, findAttr(rec, "error"), "'error' attribute should not be present on success")
}

func TestLoggingInterceptor_FailsWithGRPCStatus(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	expectedCode := codes.NotFound
	err := status.Errorf(expectedCode, "resource not found")

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/NotFound",
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, err
	}

	resp, err := interceptor(context.Background(), nil, info, actualHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, expectedCode, st.Code())
	assert.Nil(t, resp)

	require.Len(t, handler.records, 1)
	rec := handler.records[0]

	assert.Equal(t, slog.LevelError, rec.Level)

	statusAttr := findAttr(rec, "grpc.status_code")
	require.NotNil(t, statusAttr)
	assert.Equal(t, "NotFound", statusAttr.Value.String())

	errAttr := findAttr(rec, "error")
	require.NotNil(t, errAttr)
	assert.NotEmpty(t, errAttr.Value.String())
}

func TestLoggingInterceptor_FailsWithNonGRPCError(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/CustomError",
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("custom error: something broke")
	}

	_, err := interceptor(context.Background(), nil, info, actualHandler)
	require.Error(t, err)

	rec := handler.records[0]
	assert.Equal(t, slog.LevelError, rec.Level)

	statusAttr := findAttr(rec, "grpc.status_code")
	require.NotNil(t, statusAttr)
	assert.Equal(t, "Unknown", statusAttr.Value.String())
}

func TestLoggingInterceptor_AccumulatesFields(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/WithFields",
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		canonicallog.AddField(ctx, "user_id", "12345")
		canonicallog.AddField(ctx, "request_id", "abc-def")
		return "success", nil
	}

	_, err := interceptor(context.Background(), nil, info, actualHandler)
	require.NoError(t, err)

	rec := handler.records[0]

	userIDAttr := findAttr(rec, "user_id")
	require.NotNil(t, userIDAttr, "'user_id' attribute should be present")
	assert.Equal(t, "12345", userIDAttr.Value.String())

	reqIDAttr := findAttr(rec, "request_id")
	require.NotNil(t, reqIDAttr, "'request_id' attribute should be present")
	assert.Equal(t, "abc-def", reqIDAttr.Value.String())
}

func TestLoggingInterceptor_MultipleAccumulatedFields(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/MultiField",
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		canonicallog.AddField(ctx, "int_field", 42)
		canonicallog.AddField(ctx, "float_field", 3.14)
		canonicallog.AddField(ctx, "bool_field", true)
		canonicallog.AddField(ctx, "string_field", "hello")
		return nil, status.Errorf(codes.InvalidArgument, "validation failed")
	}

	_, err := interceptor(context.Background(), nil, info, actualHandler)
	require.Error(t, err)

	rec := handler.records[0]

	testCases := []struct {
		key      string
		expected interface{}
	}{
		{"int_field", int64(42)},
		{"float_field", 3.14},
		{"bool_field", true},
		{"string_field", "hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			attr := findAttr(rec, tc.key)
			require.NotNil(t, attr, "attribute %q should be present", tc.key)

			switch v := tc.expected.(type) {
			case int64:
				assert.Equal(t, v, attr.Value.Int64())
			case float64:
				assert.Equal(t, v, attr.Value.Float64())
			case bool:
				assert.Equal(t, v, attr.Value.Bool())
			case string:
				assert.Equal(t, v, attr.Value.String())
			}
		})
	}
}

func TestLoggingInterceptor_DurationIsRecorded(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	interceptor := LoggingInterceptor(logger)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Slow",
	}

	actualHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "done", nil
	}

	_, err := interceptor(context.Background(), nil, info, actualHandler)
	require.NoError(t, err)

	rec := handler.records[0]
	durAttr := findAttr(rec, "duration")
	require.NotNil(t, durAttr, "'duration' attribute should be present")

	duration := durAttr.Value.Duration()
	assert.Greater(t, duration, time.Duration(0), "duration should be positive")
}
