package grpc

import (
	"context"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func recoverUnary(ctx context.Context, request any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (response any, err error) {
	defer func() {
		if recover() != nil {
			response = nil
			err = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, request)
}
func recoverStream(service any, stream ggrpc.ServerStream, info *ggrpc.StreamServerInfo, handler ggrpc.StreamHandler) (err error) {
	defer func() {
		if recover() != nil {
			err = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(service, stream)
}
