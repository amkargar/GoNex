package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"sync"
	"time"
)

type ClientOptions struct {
	Target              string
	TLS                 *tls.Config
	DevelopmentInsecure bool
	Timeout             time.Duration
	Dialer              func(context.Context, string) (net.Conn, error)
	Interceptors        []ggrpc.UnaryClientInterceptor
}

// ClientConfig is immutable. Its identity includes the target, security and
// custom transport configuration. Reuse a config to reuse its connection.
type ClientConfig struct {
	target  string
	options []ggrpc.DialOption
}

func NewClientConfig(o ClientOptions) (*ClientConfig, error) {
	if o.Target == "" {
		return nil, fmt.Errorf("gRPC client target is required")
	}
	if o.Timeout < 0 {
		return nil, fmt.Errorf("negative RPC timeout")
	}
	if o.Timeout == 0 {
		o.Timeout = 5 * time.Second
	}
	var creds credentials.TransportCredentials
	if o.TLS != nil {
		cfg := o.TLS.Clone()
		if cfg.InsecureSkipVerify {
			return nil, fmt.Errorf("TLS certificate verification must be enabled")
		}
		if cfg.MinVersion < tls.VersionTLS12 {
			cfg.MinVersion = tls.VersionTLS12
		}
		if cfg.MaxVersion != 0 && cfg.MaxVersion < cfg.MinVersion {
			return nil, fmt.Errorf("invalid TLS protocol range")
		}
		if cfg.RootCAs != nil {
			cfg.RootCAs = cfg.RootCAs.Clone()
		}
		creds = credentials.NewTLS(cfg)
	} else if o.DevelopmentInsecure {
		creds = insecure.NewCredentials()
	} else {
		return nil, fmt.Errorf("gRPC client requires TLS or explicit DevelopmentInsecure")
	}
	deadline := func(ctx context.Context, method string, req, reply any, cc *ggrpc.ClientConn, invoke ggrpc.UnaryInvoker, opts ...ggrpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, o.Timeout)
		defer cancel()
		return invoke(ctx, method, req, reply, cc, opts...)
	}
	opts := []ggrpc.DialOption{ggrpc.WithTransportCredentials(creds), ggrpc.WithChainUnaryInterceptor(append([]ggrpc.UnaryClientInterceptor{deadline}, o.Interceptors...)...), ggrpc.WithChainStreamInterceptor(streamClientDeadline(o.Timeout)), ggrpc.WithDisableRetry()}
	if o.Dialer != nil {
		opts = append(opts, ggrpc.WithContextDialer(o.Dialer))
	}
	return &ClientConfig{target: o.Target, options: opts}, nil
}

type Clients struct {
	mu          sync.Mutex
	connections map[*ClientConfig]*ggrpc.ClientConn
	closed      bool
}

func NewClients() *Clients { return &Clients{connections: map[*ClientConfig]*ggrpc.ClientConn{}} }
func (m *Clients) Start(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("gRPC clients are closed")
	}
	return nil
}
func (m *Clients) Conn(config *ClientConfig) (*ggrpc.ClientConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || config == nil {
		return nil, fmt.Errorf("gRPC clients closed or configuration missing")
	}
	if conn := m.connections[config]; conn != nil {
		return conn, nil
	}
	conn, err := ggrpc.NewClient(config.target, config.options...)
	if err != nil {
		return nil, fmt.Errorf("invalid gRPC target configuration")
	}
	m.connections[config] = conn
	return conn, nil
}
func (m *Clients) Ready(ctx context.Context, config *ClientConfig) error {
	conn, err := m.Conn(config)
	if err != nil {
		return err
	}
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return fmt.Errorf("gRPC connection closed")
		}
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}
func (m *Clients) Close(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var first error
	for _, conn := range m.connections {
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func Client[T any](manager *Clients, config *ClientConfig, constructor func(ggrpc.ClientConnInterface) T) (T, error) {
	var zero T
	if manager == nil || constructor == nil {
		return zero, fmt.Errorf("typed client needs manager and constructor")
	}
	conn, err := manager.Conn(config)
	if err != nil {
		return zero, err
	}
	return constructor(conn), nil
}

// Registry resolves service names to explicit immutable client configurations.
type Registry map[string]*ClientConfig

func (r Registry) Resolve(name string) (*ClientConfig, error) {
	config := r[name]
	if config == nil {
		return nil, fmt.Errorf("gRPC service %q is not configured", name)
	}
	return config, nil
}
