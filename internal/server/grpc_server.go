package server

import (
	"context"
	"time"

	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

type StatusServiceImpl struct {
	v1.UnimplementedStatusServiceServer
}

func (s *StatusServiceImpl) Echo(ctx context.Context, req *v1.EchoRequest) (*v1.EchoResponse, error) {
	timestamp := time.Now().Unix()
	return &v1.EchoResponse{
		Message:   req.GetMessage(),
		Timestamp: timestamp,
	}, nil
}
