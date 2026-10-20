// Run with go run ./examples/observability, then follow its README.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/observability"
	exporter "github.com/amkargar/gonex/observability/prometheus"
	"github.com/gin-gonic/gin"
)

type module struct{ metrics *observability.Metrics }

func (m module) Register(r *app.Registrar) error {
	if err := r.Manage(m.metrics); err != nil {
		return err
	}
	if err := r.Check("dependency", func(context.Context) error {
		if _, err := os.Stat("dependency.down"); err == nil {
			return errors.New("dependency unavailable")
		}
		return nil
	}); err != nil {
		return err
	}
	handler, err := exporter.Handler(m.metrics)
	if err != nil {
		return err
	}
	if err = r.Handle("GET", "/metrics", gin.WrapH(handler)); err != nil {
		return err
	}
	return r.Handle("GET", "/work", func(c *gin.Context) {
		if _, err := os.Stat("dependency.down"); err == nil {
			observability.Logger(c.Request.Context(), nil).ErrorContext(c.Request.Context(), "dependency unavailable", "module", "demo")
			c.AbortWithStatusJSON(503, gin.H{"code": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
func main() {
	gin.SetMode(gin.ReleaseMode)
	metrics := observability.NewMetrics()
	a, err := gonex.New(config.Config{Name: "diagnostics", Middleware: []gin.HandlerFunc{observability.HTTP(metrics)}})
	if err != nil {
		log.Fatal(err)
	}
	metrics.SetReadiness(a.Ready)
	if err = a.Register(module{metrics}); err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = a.Run(ctx); err != nil {
		os.Exit(1)
	}
}
