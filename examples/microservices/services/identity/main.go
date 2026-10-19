package main

import (
	"context"
	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	usersgrpc "github.com/amkargar/gonex/examples/microservices/services/identity/modules/users/grpc/v1"
	rpc "github.com/amkargar/gonex/grpc"
	"github.com/amkargar/gonex/observability"
	observedgrpc "github.com/amkargar/gonex/observability/grpc"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	tlsConfig, err := rpc.ServerTLS(os.Getenv("GRPC_TLS_CERT"), os.Getenv("GRPC_TLS_KEY"), os.Getenv("GRPC_CLIENT_CA"))
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("GRPC_ADDRESS")
	if address == "" {
		address = "127.0.0.1:9091"
	}
	metrics := observability.NewMetrics()
	server, err := rpc.NewServer(rpc.ServerOptions{Address: address, TLS: tlsConfig, DevelopmentInsecure: os.Getenv("GONEX_GRPC_INSECURE") == "1", Interceptors: []grpc.UnaryServerInterceptor{observedgrpc.Server(metrics)}})
	if err != nil {
		log.Fatal(err)
	}
	if err = server.Register(func(s *grpc.Server) error { pb.RegisterIdentityServiceServer(s, usersgrpc.Server{}); return nil }); err != nil {
		log.Fatal(err)
	}
	a, err := gonex.New(config.Config{DisableHTTP: true})
	if err != nil {
		log.Fatal(err)
	}
	if err = a.Register(resources{[]app.Resource{metrics, server}}); err != nil {
		log.Fatal(err)
	}
	if err = a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

type resources struct{ values []app.Resource }

func (m resources) Register(r *app.Registrar) error {
	for _, resource := range m.values {
		if err := r.Manage(resource); err != nil {
			return err
		}
	}
	return nil
}
