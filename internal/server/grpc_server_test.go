package server

import (
	"context"
	"testing"
	"time"

	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/stretchr/testify/assert"
)

func TestEcho(t *testing.T) {
	tests := []struct {
		name        string
		input       *v1.EchoRequest
		expectError bool
	}{
		{
			name: "with message",
			input: &v1.EchoRequest{
				Message: "hello",
			},
			expectError: false,
		},
		{
			name: "empty message",
			input: &v1.EchoRequest{
				Message: "",
			},
			expectError: false,
		},
		{
			name: "long message",
			input: &v1.EchoRequest{
				Message: "this is a pretty long message for testing purposes",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impl := &StatusServiceImpl{}

			ctx := context.Background()
			resp, err := impl.Echo(ctx, tt.input)

			assert := assert.New(t)

			if !tt.expectError {
				assert.NoError(err)
				assert.Equal(tt.input.Message, resp.Message)
				assert.NotZero(resp.Timestamp)
				assert.True(resp.Timestamp >= time.Now().Add(-1*time.Second).Unix())
				assert.True(resp.Timestamp <= time.Now().Add(1*time.Second).Unix())
			} else {
				assert.Error(err)
				assert.Nil(resp)
			}
		})
	}
}

func TestServiceImplImplementsInterface(t *testing.T) {
	var _ v1.StatusServiceServer = &StatusServiceImpl{}
	assert.True(t, true, "StatusServiceImpl implements StatusServiceServer")
}