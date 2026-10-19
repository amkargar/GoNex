// Package grpc provides optional managed gRPC servers and clients.
package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"sync"
	"time"
)

type ServerOptions struct {
	Address             string
	TLS                 *tls.Config
	DevelopmentInsecure bool
	Timeout             time.Duration
	Interceptors        []ggrpc.UnaryServerInterceptor
}
type Server struct {
	mu      sync.Mutex
	server  *ggrpc.Server
	health  *health.Server
	options ServerOptions
	address string
	state   string
	errors  chan error
	done    chan struct{}
}

func NewServer(o ServerOptions) (*Server, error) {
	if o.Address == "" {
		o.Address = "127.0.0.1:9090"
	}
	if o.Timeout < 0 {
		return nil, fmt.Errorf("negative RPC timeout")
	}
	if o.Timeout == 0 {
		o.Timeout = 5 * time.Second
	}
	opts := []ggrpc.ServerOption{ggrpc.ChainUnaryInterceptor(append([]ggrpc.UnaryServerInterceptor{recoverUnary, serverDeadline(o.Timeout)}, o.Interceptors...)...), ggrpc.ChainStreamInterceptor(recoverStream, streamServerDeadline(o.Timeout))}
	if o.TLS != nil {
		cfg := o.TLS.Clone()
		if cfg.MinVersion < tls.VersionTLS12 {
			cfg.MinVersion = tls.VersionTLS12
		}
		if cfg.MaxVersion != 0 && cfg.MaxVersion < cfg.MinVersion {
			return nil, fmt.Errorf("invalid TLS protocol range")
		}
		for _, certificate := range cfg.Certificates {
			if certificate.PrivateKey == nil || len(certificate.Certificate) == 0 {
				return nil, fmt.Errorf("invalid TLS server certificate/key")
			}
		}
		if cfg.ClientCAs != nil {
			cfg.ClientCAs = cfg.ClientCAs.Clone()
		}
		if len(cfg.Certificates) == 0 && cfg.GetCertificate == nil {
			return nil, fmt.Errorf("gRPC TLS needs a server certificate")
		}
		if cfg.ClientAuth >= tls.VerifyClientCertIfGiven && cfg.ClientCAs == nil {
			return nil, fmt.Errorf("mTLS needs client trust roots")
		}
		opts = append(opts, ggrpc.Creds(credentials.NewTLS(cfg)))
	} else if !o.DevelopmentInsecure {
		return nil, fmt.Errorf("gRPC requires TLS; DevelopmentInsecure is explicit development-only opt-in")
	}
	s := &Server{server: ggrpc.NewServer(opts...), health: health.NewServer(), options: o, state: "created", errors: make(chan error, 1), done: make(chan struct{})}
	healthpb.RegisterHealthServer(s.server, s.health)
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	return s, nil
}
func (s *Server) Register(register func(*ggrpc.Server) error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "created" || register == nil {
		return fmt.Errorf("gRPC registration is closed or invalid")
	}
	defer func() {
		if recover() != nil {
			s.state = "failed"
			err = fmt.Errorf("gRPC service registration conflict")
		}
		if err != nil {
			s.state = "failed"
		}
	}()
	return register(s.server)
}
func (s *Server) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "created" {
		return fmt.Errorf("cannot start gRPC in state %s", s.state)
	}
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.options.Address)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	s.address = l.Addr().String()
	s.state = "running"
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	go func() {
		if err := s.server.Serve(l); err != nil {
			s.errors <- err
		}
	}()
	return nil
}
func (s *Server) Address() string      { s.mu.Lock(); defer s.mu.Unlock(); return s.address }
func (s *Server) Errors() <-chan error { return s.errors }
func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.state == "stopping" || s.state == "stopped" {
		done := s.done
		s.mu.Unlock()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.state = "stopping"
	s.health.Shutdown()
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.server.GracefulStop(); close(done) }()
	var err error
	select {
	case <-done:
	case <-ctx.Done():
		s.server.Stop()
		<-done
		err = ctx.Err()
	}
	s.mu.Lock()
	s.state = "stopped"
	close(s.done)
	s.mu.Unlock()
	return err
}

func serverDeadline(timeout time.Duration) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}
