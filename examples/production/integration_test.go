package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/gonex-production/migrations"
	"example.com/gonex-production/modules/identity"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/orm"
)

func TestProductionAPI(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orders.db")) + "?_pragma=busy_timeout(3000)"
			if dialect == "postgres" {
				dsn = os.Getenv("GONEX_PRODUCTION_TEST_DSN")
				if dsn == "" {
					if os.Getenv("GONEX_REQUIRE_POSTGRES") == "1" {
						t.Fatal("required production-test DSN missing")
					}
					t.Skip("disposable PostgreSQL required")
				}
				admin, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSN: dsn})
				if err != nil {
					t.Fatal(err)
				}
				pool, _ := admin.SQL()
				schema := fmt.Sprintf("sample_%d", time.Now().UnixNano())
				if _, err = pool.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
					defer stop()
					_, _ = pool.ExecContext(cleanup, `DROP SCHEMA "`+schema+`" CASCADE`)
					admin.Close(cleanup)
				})
				u, err := url.Parse(dsn)
				if err != nil || u.Host == "" {
					t.Fatal("test requires URL DSN")
				}
				q := u.Query()
				q.Set("search_path", schema)
				u.RawQuery = q.Encode()
				dsn = u.String()
			}
			db, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSN: dsn})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close(ctx)
			pool, _ := db.SQL()
			runner, err := migration.New(pool, dialect, migrations.Files)
			if err != nil {
				t.Fatal(err)
			}
			if err = runner.Up(ctx); err != nil {
				t.Fatal(err)
			}
			adminToken, readerToken := strings.Repeat("a", 40), strings.Repeat("r", 40)
			policy, err := identity.New(adminToken, readerToken)
			if err != nil {
				t.Fatal(err)
			}
			a, err := application(db, policy, config.Config{Address: "127.0.0.1:0", Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			if err = a.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer a.Shutdown(context.Background())
			// CRUD contract checks are not a runner latency SLO. Windows hosted
			// runners may pause longer than that budget; keep a bounded
			// client budget independent of the application's two-second deadline.
			client := http.Client{Timeout: 10 * time.Second}
			request := func(method, path, body, token string, status int) map[string]any {
				t.Helper()
				r, err := http.NewRequestWithContext(ctx, method, "http://"+a.Address()+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				if token != "" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				if body != "" {
					r.Header.Set("Content-Type", "application/json")
				}
				res, err := client.Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				raw, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != status {
					t.Fatalf("%s %s got %d want %d: %s", method, path, res.StatusCode, status, raw)
				}
				if strings.Contains(string(raw), adminToken) || strings.Contains(string(raw), readerToken) {
					t.Fatal("token leaked")
				}
				if status == 204 {
					return nil
				}
				var result map[string]any
				if err = json.Unmarshal(raw, &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			request("GET", "/readyz", "", "", 200)
			request("GET", "/api/orders", "", "", 401)
			request("POST", "/api/orders", `{"item":"Book","quantity":2}`, readerToken, 403)
			request("POST", "/api/orders", `{"item":"A","quantity":0}`, adminToken, 422)
			created := request("POST", "/api/orders", `{"item":"Book","quantity":2}`, adminToken, 201)
			id := created["id"].(string)
			request("GET", "/api/orders/"+id, "", readerToken, 200)
			list := request("GET", "/api/orders?limit=1&offset=0&status=pending", "", readerToken, 200)
			if len(list["items"].([]any)) != 1 {
				t.Fatal(list)
			}
			request("GET", "/api/orders?limit=100", "", readerToken, 422)
			request("GET", "/api/orders?status=unknown", "", readerToken, 400)
			request("PUT", "/api/orders/"+id, `{"item":"Updated book","quantity":3,"status":"completed"}`, adminToken, 200)
			list = request("GET", "/api/orders?status=pending", "", readerToken, 200)
			if len(list["items"].([]any)) != 0 {
				t.Fatal(list)
			}
			request("DELETE", "/api/orders/"+id, "", adminToken, 204)
			request("GET", "/api/orders/"+id, "", readerToken, 404)
			doc := request("GET", "/openapi.json", "", "", 200)
			if doc["components"] == nil || doc["paths"] == nil {
				t.Fatal(doc)
			}
			// A closed pool must not leak connection details and must fail readiness.
			if err = db.Close(ctx); err != nil {
				t.Fatal(err)
			}
			request("GET", "/readyz", "", "", 503)
			request("GET", "/api/orders", "", readerToken, 503)
		})
	}
}
