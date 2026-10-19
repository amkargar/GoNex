package grpc

import (
	"context"
	ggrpc "google.golang.org/grpc"
	"time"
)

type serverStream struct {
	ggrpc.ServerStream
	ctx context.Context
}

func (s serverStream) Context() context.Context { return s.ctx }
func streamServerDeadline(timeout time.Duration) ggrpc.StreamServerInterceptor {
	return func(service any, stream ggrpc.ServerStream, info *ggrpc.StreamServerInfo, handler ggrpc.StreamHandler) error {
		ctx, cancel := context.WithTimeout(stream.Context(), timeout)
		defer cancel()
		return handler(service, serverStream{ServerStream: stream, ctx: ctx})
	}
}

type clientStream struct {
	ggrpc.ClientStream
	cancel context.CancelFunc
}

func (s clientStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	if err != nil {
		s.cancel()
	}
	return err
}
func (s clientStream) SendMsg(message any) error {
	err := s.ClientStream.SendMsg(message)
	if err != nil {
		s.cancel()
	}
	return err
}
func streamClientDeadline(timeout time.Duration) ggrpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *ggrpc.StreamDesc, conn *ggrpc.ClientConn, method string, streamer ggrpc.Streamer, options ...ggrpc.CallOption) (ggrpc.ClientStream, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		stream, err := streamer(ctx, desc, conn, method, options...)
		if err != nil {
			cancel()
			return nil, err
		}
		return clientStream{ClientStream: stream, cancel: cancel}, nil
	}
}
