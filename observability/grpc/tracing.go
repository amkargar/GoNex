// Package grpc adds optional gRPC trace propagation and metrics interceptors.
package grpc

import (
	"context"
	"github.com/amkargar/gonex/observability"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func Server(metrics *observability.Metrics) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (response any, err error) {
		incoming := ""
		values := metadata.ValueFromIncomingContext(ctx, "traceparent")
		if len(values) == 1 {
			incoming = values[0]
		}
		ctx = observability.WithTraceparent(ctx, incoming)
		finish := func(bool) {}
		if metrics != nil {
			finish = metrics.Begin("gRPC server " + info.FullMethod)
		}
		defer func() {
			if fault := recover(); fault != nil {
				finish(true)
				panic(fault)
			}
			finish(err != nil)
		}()
		return handler(ctx, request)
	}
}
func Client(metrics *observability.Metrics) ggrpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, response any, connection *ggrpc.ClientConn, invoke ggrpc.UnaryInvoker, options ...ggrpc.CallOption) error {
		trace := observability.Traceparent(ctx)
		if !observability.ValidTraceparent(trace) {
			ctx = observability.WithTraceparent(ctx, "")
			trace = observability.Traceparent(ctx)
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		md.Set("traceparent", trace)
		ctx = metadata.NewOutgoingContext(ctx, md)
		finish := func(bool) {}
		if metrics != nil {
			finish = metrics.Begin("gRPC client " + method)
		}
		err := invoke(ctx, method, request, response, connection, options...)
		finish(err != nil)
		return err
	}
}
