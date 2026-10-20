package tests

import (
	"context"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/basic/migrations"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/orm"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

type benchmarkResponse struct {
	Message string `json:"message"`
}

func BenchmarkComparisonHTTP(b *testing.B) {
	bare := gin.New()
	bare.GET("/bench", func(c *gin.Context) { c.JSON(200, benchmarkResponse{Message: "hello"}) })
	a, e := app.New(config.Config{DisableOpenAPI: true, DisableRequestLogging: true})
	if e != nil {
		b.Fatal(e)
	}
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		return transport.Register(r, transport.Endpoint[transport.NoBody, benchmarkResponse]{Method: "GET", Path: "/bench", OperationID: "benchmark", Status: 200, Handler: func(context.Context, transport.NoBody) (benchmarkResponse, error) {
			return benchmarkResponse{Message: "hello"}, nil
		}})
	})); e != nil {
		b.Fatal(e)
	}
	framework, e := a.Handler()
	if e != nil {
		b.Fatal(e)
	}
	request := httptest.NewRequest("GET", "/bench", nil)
	b.Run("Gin", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			bare.ServeHTTP(w, request)
			if w.Code != 200 {
				b.Fatal(w.Code)
			}
		}
	})
	b.Run("Gonex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			framework.ServeHTTP(w, request)
			if w.Code != 200 {
				b.Fatal(w.Code)
			}
		}
	})
}

func BenchmarkComparisonDatabase(b *testing.B) {
	ctx := context.Background()
	database, e := orm.Open(ctx, orm.Options{Dialect: "sqlite", DSN: "file:" + filepath.ToSlash(filepath.Join(b.TempDir(), "benchmark.db"))})
	if e != nil {
		b.Fatal(e)
	}
	defer database.Close(ctx)
	pool, _ := database.SQL()
	runner, e := migration.New(pool, "sqlite", migrations.Files)
	if e != nil {
		b.Fatal(e)
	}
	if e = runner.Up(ctx); e != nil {
		b.Fatal(e)
	}
	repository := users.NewGORMRepository(database)
	user, e := repository.Create(ctx, "Ada", "ada@example.com")
	if e != nil {
		b.Fatal(e)
	}
	db, _ := database.DB()
	b.Run("GORM", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var model users.UserModel
			if e := db.WithContext(ctx).First(&model, "id = ?", user.ID).Error; e != nil || model.ID != user.ID {
				b.Fatal(e)
			}
		}
	})
	b.Run("Gonex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			got, e := repository.Get(ctx, user.ID)
			if e != nil || got.ID != user.ID {
				b.Fatal(e)
			}
		}
	})
}
