package main

import (
	"context"
	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/microservices/services/orders/modules/orders"
	rpc "github.com/amkargar/gonex/grpc"
	"github.com/amkargar/gonex/observability"
	observedgrpc "github.com/amkargar/gonex/observability/grpc"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	tlsConfig, err := rpc.ClientTLS(os.Getenv("CLIENTS_IDENTITY_CA"), os.Getenv("CLIENTS_IDENTITY_SERVER_NAME"), os.Getenv("CLIENTS_IDENTITY_CERT"), os.Getenv("CLIENTS_IDENTITY_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("CLIENTS_IDENTITY_ADDRESS")
	if address == "" {
		address = "127.0.0.1:9091"
	}
	metrics := observability.NewMetrics()
	pool := rpc.NewClients()
	clientConfig, err := rpc.NewClientConfig(rpc.ClientOptions{Target: address, TLS: tlsConfig, DevelopmentInsecure: os.Getenv("CLIENTS_IDENTITY_INSECURE") == "1", Interceptors: []grpc.UnaryClientInterceptor{observedgrpc.Client(metrics)}})
	if err != nil {
		log.Fatal(err)
	}
	identity, err := orders.NewIdentityClient(pool, clientConfig)
	if err != nil {
		log.Fatal(err)
	}
	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = "127.0.0.1:8082"
	}
	a, err := gonex.New(config.Config{Address: httpAddress, Middleware: []gin.HandlerFunc{observability.HTTP(metrics)}})
	if err != nil {
		log.Fatal(err)
	}
	if err = a.Register(dependencies{pool, metrics, clientConfig}, orders.Module{Service: orders.Service{Users: identity}}); err != nil {
		log.Fatal(err)
	}
	if err = a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

type dependencies struct {
	pool    *rpc.Clients
	metrics *observability.Metrics
	config  *rpc.ClientConfig
}

func (d dependencies) Register(r *app.Registrar) error {
	if err := r.Manage(d.pool); err != nil {
		return err
	}
	if err := r.Manage(d.metrics); err != nil {
		return err
	}
	return r.Check("identity", func(ctx context.Context) error { return d.pool.Ready(ctx, d.config) })
}
