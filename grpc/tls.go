package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func trustRoots(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read TLS trust roots")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("invalid TLS trust roots")
	}
	return pool, nil
}
func ServerTLS(cert, key, clientCA string) (*tls.Config, error) {
	if cert == "" && key == "" && clientCA == "" {
		return nil, nil
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS server certificate/key")
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	if clientCA != "" {
		cfg.ClientCAs, err = trustRoots(clientCA)
		if err != nil {
			return nil, err
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}
func ClientTLS(ca, serverName, cert, key string) (*tls.Config, error) {
	if ca == "" && serverName == "" && cert == "" && key == "" {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	var err error
	if ca != "" {
		cfg.RootCAs, err = trustRoots(ca)
		if err != nil {
			return nil, err
		}
	}
	if cert != "" || key != "" {
		pair, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS client certificate/key")
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}
