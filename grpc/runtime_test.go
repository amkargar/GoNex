package grpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	rpc "github.com/amkargar/gonex/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	pb "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/status"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testServer struct {
	pb.UnimplementedTestServiceServer
	calls atomic.Int32
}

func (s *testServer) UnaryCall(ctx context.Context, in *pb.SimpleRequest) (*pb.SimpleResponse, error) {
	s.calls.Add(1)
	if string(in.GetPayload().GetBody()) == "slow" {
		<-ctx.Done()
		return nil, rpc.ToStatus(ctx.Err())
	}
	return nil, status.Error(codes.Unavailable, "dependency unavailable")
}
func start(t *testing.T, tlsConfig *tls.Config) (*rpc.Server, *testServer) {
	t.Helper()
	server, e := rpc.NewServer(rpc.ServerOptions{Address: "127.0.0.1:0", TLS: tlsConfig, DevelopmentInsecure: tlsConfig == nil})
	if e != nil {
		t.Fatal(e)
	}
	service := &testServer{}
	if e = server.Register(func(s *grpc.Server) error { pb.RegisterTestServiceServer(s, service); return nil }); e != nil {
		t.Fatal(e)
	}
	if e = server.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})
	return server, service
}

func TestClientsReuseDeadlineAndClose(t *testing.T) {
	server, service := start(t, nil)
	config, e := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), DevelopmentInsecure: true, Timeout: 30 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	pool := rpc.NewClients()
	defer pool.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if e = pool.Ready(ctx, config); e != nil {
		t.Fatal(e)
	}
	conn, e := pool.Conn(config)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Go(func() {
			other, e := pool.Conn(config)
			if e != nil || other != conn {
				t.Error("connection was not reused", e)
			}
		})
	}
	wg.Wait()
	health, e := rpc.Client(pool, config, healthpb.NewHealthClient)
	if e != nil {
		t.Fatal(e)
	}
	response, e := health.Check(ctx, &healthpb.HealthCheckRequest{})
	if e != nil || response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatal(response, e)
	}
	client, e := rpc.Client(pool, config, pb.NewTestServiceClient)
	if e != nil {
		t.Fatal(e)
	}
	_, e = client.UnaryCall(ctx, &pb.SimpleRequest{})
	if status.Code(e) != codes.Unavailable || service.calls.Load() != 1 {
		t.Fatal("unexpected retry", service.calls.Load(), e)
	}
	short, stop := context.WithTimeout(ctx, 5*time.Millisecond)
	defer stop()
	began := time.Now()
	_, e = client.UnaryCall(short, &pb.SimpleRequest{Payload: &pb.Payload{Body: []byte("slow")}})
	if status.Code(e) != codes.DeadlineExceeded || time.Since(began) > 300*time.Millisecond {
		t.Fatal(e)
	}
	_, e = client.UnaryCall(ctx, &pb.SimpleRequest{Payload: &pb.Payload{Body: []byte("slow")}})
	if status.Code(e) != codes.DeadlineExceeded {
		t.Fatal(e)
	}
	different, _ := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), DevelopmentInsecure: true})
	other, _ := pool.Conn(different)
	if other == conn {
		t.Fatal("different immutable configurations shared a connection")
	}
	if e = server.Close(ctx); e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", server.Address())
	if e != nil {
		t.Fatal("listener not released", e)
	}
	l.Close()
	if e = pool.Close(ctx); e != nil {
		t.Fatal(e)
	}
	if conn.GetState() != connectivity.Shutdown {
		t.Fatal("connection stayed open")
	}
	if _, e = pool.Conn(config); e == nil {
		t.Fatal("closed manager accepted a client")
	}
}

func certificates(t *testing.T) (tls.Certificate, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caPub, caKey, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Gonex test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, caPub, caKey)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	caParsed, e := x509.ParseCertificate(caDER)
	if e != nil {
		t.Fatal(e)
	}
	roots.AddCert(caParsed)
	issue := func(serial int64, usage x509.ExtKeyUsage) tls.Certificate {
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, e := x509.CreateCertificate(rand.Reader, cert, caParsed, pub, caKey)
		if e != nil {
			t.Fatal(e)
		}
		return tls.Certificate{Certificate: [][]byte{der, caDER}, PrivateKey: key}
	}
	return issue(2, x509.ExtKeyUsageServerAuth), issue(3, x509.ExtKeyUsageClientAuth), roots
}

func TestTLSAndMTLS(t *testing.T) {
	serverCert, clientCert, roots := certificates(t)
	server, _ := start(t, &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})
	pool := rpc.NewClients()
	defer pool.Close(context.Background())
	config, e := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), TLS: &tls.Config{RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}}, Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = pool.Ready(ctx, config); e != nil {
		t.Fatal(e)
	}
	client, e := rpc.Client(pool, config, healthpb.NewHealthClient)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = client.Check(ctx, &healthpb.HealthCheckRequest{}); e != nil {
		t.Fatal(e)
	}
	bad, e := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), TLS: &tls.Config{RootCAs: roots, ServerName: "localhost"}})
	if e != nil {
		t.Fatal(e)
	}
	short, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	if e = pool.Ready(short, bad); e == nil {
		t.Fatal("mTLS accepted a client without a certificate")
	}
	if _, e = rpc.NewServer(rpc.ServerOptions{}); e == nil {
		t.Fatal("insecure default accepted")
	}
	if _, e = rpc.NewServer(rpc.ServerOptions{TLS: &tls.Config{}}); e == nil {
		t.Fatal("missing certificate accepted")
	}
	if _, e = rpc.NewClientConfig(rpc.ClientOptions{Target: "localhost:1"}); e == nil {
		t.Fatal("insecure client accepted")
	}
	if _, e = rpc.NewClientConfig(rpc.ClientOptions{Target: "localhost:1", TLS: &tls.Config{InsecureSkipVerify: true}}); e == nil {
		t.Fatal("verification bypass accepted")
	}
}
