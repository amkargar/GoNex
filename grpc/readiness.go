package grpc

import (
	"context"
	"fmt"
)

func (s *Server) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "running" {
		return fmt.Errorf("gRPC server is not ready")
	}
	return nil
}
