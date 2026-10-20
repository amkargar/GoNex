package main

import (
	"context"
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/gonex-production/migrations"
	"example.com/gonex-production/modules/identity"
	"example.com/gonex-production/modules/orders"
	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/observability"
	exporter "github.com/amkargar/gonex/observability/prometheus"
	"github.com/amkargar/gonex/orm"
	"github.com/gin-gonic/gin"
)

type resources struct {
	db      *orm.Database
	metrics *observability.Metrics
}

func (r resources) Register(reg *app.Registrar) error {
	if err := reg.Manage(r.db); err != nil {
		return err
	}
	if err := reg.Manage(r.metrics); err != nil {
		return err
	}
	if os.Getenv("GONEX_EXPOSE_METRICS") == "1" {
		handler, err := exporter.Handler(r.metrics)
		if err != nil {
			return err
		}
		return reg.Handle("GET", "/metrics", gin.WrapH(handler))
	}
	return nil
}
func application(db *orm.Database, policy identity.Policy, settings config.Config) (*app.App, error) {
	deadline, err := transport.Deadline(2 * time.Second)
	if err != nil {
		return nil, err
	}
	metrics := observability.NewMetrics()
	settings.Middleware = append(settings.Middleware, deadline, observability.HTTP(metrics))
	a, err := gonex.New(settings)
	if err != nil {
		return nil, err
	}
	metrics.SetReadiness(a.Ready)
	if err = a.Register(resources{db, metrics}, orders.Module{Service: orders.NewService(orders.Repository{Database: db}), Policy: policy}); err != nil {
		return nil, err
	}
	return a, nil
}
func execute(ctx context.Context) error {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		address := os.Getenv("HTTP_ADDRESS")
		if address == "" {
			address = "127.0.0.1:8080"
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("invalid healthcheck address")
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "localhost"
		}
		client := http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/readyz")
		if err != nil {
			return fmt.Errorf("service not ready")
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		if response.StatusCode != 200 {
			return fmt.Errorf("service not ready")
		}
		return nil
	}

	dialect := os.Getenv("DATABASE_DIALECT")
	if dialect == "" {
		dialect = "postgres"
	}
	db, err := orm.New(orm.Options{Dialect: dialect, DSNEnv: "DATABASE_URL"})
	if err != nil {
		return err
	}
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		if err = db.Start(ctx); err != nil {
			return err
		}
		defer db.Close(ctx)
		pool, _ := db.SQL()
		runner, err := migration.New(pool, dialect, migrations.Files)
		if err != nil {
			return err
		}
		return runner.Up(ctx)
	}
	if len(os.Args) != 1 {
		return fmt.Errorf("usage: api [migrate|healthcheck]")
	}
	policy, err := identity.New(os.Getenv("GONEX_ADMIN_TOKEN"), os.Getenv("GONEX_READER_TOKEN"))
	if err != nil {
		return err
	}
	a, err := application(db, policy, config.Config{Name: "gonex-orders", Address: os.Getenv("HTTP_ADDRESS"), Environment: os.Getenv("GONEX_ENV"), LogFormat: os.Getenv("GONEX_LOG_FORMAT"), LogLevel: os.Getenv("GONEX_LOG_LEVEL"), DisableOpenAPI: os.Getenv("GONEX_DISABLE_OPENAPI") == "1"})
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
func main() {
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}
