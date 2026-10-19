package tests

import (
	"context"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	rpc "github.com/amkargar/gonex/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"testing"
	"time"
)

func TestGRPCOnlyManagedLifecycle(t *testing.T) {
	server, e := rpc.NewServer(rpc.ServerOptions{Address: "127.0.0.1:0", DevelopmentInsecure: true})
	if e != nil {
		t.Fatal(e)
	}
	pool := rpc.NewClients()
	a, e := app.New(config.Config{DisableHTTP: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := r.Manage(server); e != nil {
			return e
		}
		return r.Manage(pool)
	})); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = a.Start(ctx); e != nil {
		t.Fatal(e)
	}
	if a.Address() != "" {
		t.Fatal("gRPC-only runtime bound HTTP")
	}
	clientConfig, e := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), DevelopmentInsecure: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = pool.Ready(ctx, clientConfig); e != nil {
		t.Fatal(e)
	}
	client, e := rpc.Client(pool, clientConfig, healthpb.NewHealthClient)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = client.Check(ctx, &healthpb.HealthCheckRequest{}); e != nil {
		t.Fatal(e)
	}
	if e = a.Shutdown(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Conn(clientConfig); e == nil {
		t.Fatal("app did not close clients")
	}
}
