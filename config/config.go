// Package config defines explicit application settings.
package config

import (
	"fmt"
	"github.com/amkargar/gonex/observability"
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

type Config struct {
	Name, Environment                                       string
	Logger                                                  *slog.Logger
	LogLevel, LogFormat, LogColor                           string
	DisableRequestLogging                                   bool
	DisableStartupBanner                                    bool
	Middleware                                              []gin.HandlerFunc
	DisableHTTP                                             bool
	DisableOpenAPI                                          bool
	DocsEnabled                                             bool
	Address                                                 string
	ReadTimeout, WriteTimeout, IdleTimeout, ShutdownTimeout time.Duration
}

func (c Config) Normalize() (Config, error) {
	if c.Name == "" {
		c.Name = "Gonex"
	}
	if c.Environment == "" {
		c.Environment = "development"
	}
	if c.Logger == nil {
		var err error
		c.Logger, err = observability.NewLogger(observability.LogOptions{Environment: c.Environment, Level: c.LogLevel, Format: c.LogFormat, Color: c.LogColor})
		if err != nil {
			return c, err
		}
	}
	for _, middleware := range c.Middleware {
		if middleware == nil {
			return c, fmt.Errorf("nil HTTP middleware")
		}
	}
	c.Middleware = append([]gin.HandlerFunc(nil), c.Middleware...)
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
