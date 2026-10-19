// Package config defines explicit application settings.
package config

import (
	"fmt"
	"time"
)

type Config struct {
	DisableOpenAPI                                          bool
	DocsEnabled                                             bool
	Address                                                 string
	ReadTimeout, WriteTimeout, IdleTimeout, ShutdownTimeout time.Duration
}

func (c Config) Normalize() (Config, error) {
	if c.Address == "" {
		c.Address = "127.0.0.1:8080"
	}
	for _, d := range []time.Duration{c.ReadTimeout, c.WriteTimeout, c.IdleTimeout, c.ShutdownTimeout} {
		if d < 0 {
			return c, fmt.Errorf("timeouts must not be negative")
		}
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 10 * time.Second
	}
	return c, nil
}
