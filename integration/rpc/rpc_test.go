package rpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"testing"
	"time"
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	apperrors "github.com/amkargar/gonex/errors"
	rpc "github.com/amkargar/gonex/grpc"
	identitygrpc "github.com/amkargar/gonex/integration/rpc/identity/grpc/v1"
	"github.com/amkargar/gonex/integration/rpc/orders"
	"google.golang.org/grpc"
)

func TestIdentityOrdersOverTLS(t *testing.T){
	pub,key,e:=ed25519.GenerateKey(rand.Reader);if e!=nil{t.Fatal(e)};cert:= &x509.Certificate{SerialNumber:big.NewInt(1),DNSNames:[]string{"identity.test"},NotBefore:time.Now().Add(-time.Hour),NotAfter:time.Now().Add(time.Hour),IsCA:true,BasicConstraintsValid:true,KeyUsage:x509.KeyUsageCertSign|x509.KeyUsageDigitalSignature,ExtKeyUsage:[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der,e:=x509.CreateCertificate(rand.Reader,cert,cert,pub,key);if e!=nil{t.Fatal(e)};parsed,e:=x509.ParseCertificate(der);if e!=nil{t.Fatal(e)};roots:=x509.NewCertPool();roots.AddCert(parsed)
	server,e:=rpc.NewServer(rpc.ServerOptions{Address:"127.0.0.1:0",TLS:&tls.Config{Certificates:[]tls.Certificate{{Certificate:[][]byte{der},PrivateKey:key}}}});if e!=nil{t.Fatal(e)}
	if e=server.Register(func(s *grpc.Server)error{pb.RegisterIdentityServiceServer(s,identitygrpc.Server{});return nil});e!=nil{t.Fatal(e)}
	if e=server.Start(context.Background());e!=nil{t.Fatal(e)};defer server.Close(context.Background())
	config,e:=rpc.NewClientConfig(rpc.ClientOptions{Target:server.Address(),TLS:&tls.Config{RootCAs:roots,ServerName:"identity.test"},Timeout:100*time.Millisecond});if e!=nil{t.Fatal(e)}
	pool:=rpc.NewClients();defer pool.Close(context.Background());ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel();if e=pool.Ready(ctx,config);e!=nil{t.Fatal(e)}
	adapter,e:=orders.NewIdentityClient(pool,config);if e!=nil{t.Fatal(e)};service:=orders.Service{Users:adapter}
	name,e:=service.UserName(ctx,"1");if e!=nil || name!="Ada"{t.Fatal(name,e)}
	_,e=service.UserName(ctx,"missing");var domain *apperrors.Error;if !errors.As(e,&domain)||domain.Code!=apperrors.NotFound{t.Fatal(e)}
	short,stop:=context.WithTimeout(ctx,10*time.Millisecond);defer stop();if _,e=service.UserName(short,"slow");!errors.Is(e,context.DeadlineExceeded){t.Fatal(e)}
	if e=server.Close(ctx);e!=nil{t.Fatal(e)};if _,e=service.UserName(ctx,"1");e==nil{t.Fatal("terminated provider appeared available")}
}
