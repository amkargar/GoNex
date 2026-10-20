package tests

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/amkargar/gonex/app"
	apperrors "github.com/amkargar/gonex/errors"
	"github.com/amkargar/gonex/examples/basic/migrations"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/orm"
	"gorm.io/gorm"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func sqliteDatabase(t *testing.T) *orm.Database {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "test.db")) + "?_pragma=busy_timeout(3000)"
	d, e := orm.Open(context.Background(), orm.Options{Dialect: "sqlite", DSN: dsn})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := d.Close(context.Background()); e != nil {
			t.Error(e)
		}
	})
	return d
}

func TestDatabaseAndTransactions(t *testing.T) {
	d := sqliteDatabase(t)
	pool, e := d.SQL()
	if e != nil {
		t.Fatal(e)
	}
	runner, e := migration.New(pool, "sqlite", migrations.Files)
	if e != nil {
		t.Fatal(e)
	}
	if e = runner.Up(context.Background()); e != nil {
		t.Fatal(e)
	}
	s := users.NewService(users.NewGORMRepository(d))
	u, e := s.Create(context.Background(), "Grace", "grace@example.com")
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.Get(context.Background(), u.ID)
	if e != nil || got != u {
		t.Fatal(got, e)
	}
	_, e = s.Create(context.Background(), "Other", "grace@example.com")
	var domain *apperrors.Error
	if !errors.As(e, &domain) || domain.Code != apperrors.Conflict {
		t.Fatalf("expected conflict, got %v", e)
	}
	_, e = s.Get(context.Background(), "missing")
	if !errors.As(e, &domain) || domain.Code != apperrors.NotFound {
		t.Fatal(e)
	}
	db, e := d.DB()
	if e != nil {
		t.Fatal(e)
	}
	e = orm.Transaction(context.Background(), db, func(tx *gorm.DB) error {
		if err := tx.Create(&users.UserModel{ID: "rollback", Name: "Rollback", Email: "rollback@example.com"}).Error; err != nil {
			return err
		}
		return tx.Create(&users.UserModel{ID: "duplicate", Name: "Duplicate", Email: u.Email}).Error
	})
	if e == nil {
		t.Fatal("transaction should fail")
	}
	if _, e = s.Get(context.Background(), "rollback"); !errors.As(e, &domain) || domain.Code != apperrors.NotFound {
		t.Fatal("first write did not roll back", e)
	}
	if e = d.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = pool.Ping(); e == nil {
		t.Fatal("pool stayed open")
	}
	if e = d.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestMigrationHistoryAndLocking(t *testing.T) {
	d := sqliteDatabase(t)
	pool, _ := d.SQL()
	runner, e := migration.New(pool, "sqlite", migrations.Files)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Go(func() { results <- runner.Up(ctx) })
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	statuses, e := runner.Status(ctx)
	if e != nil || len(statuses) != 1 || !statuses[0].Applied {
		t.Fatal(statuses, e)
	}
	changed := fstest.MapFS{"000001_create_users.up.sql": {Data: []byte("CREATE TABLE changed (id TEXT);")}}
	other, e := migration.New(pool, "sqlite", changed)
	if e != nil {
		t.Fatal(e)
	}
	if e = other.Up(ctx); e == nil || !strings.Contains(e.Error(), "checksum") {
		t.Fatal("checksum change accepted", e)
	}
	bad := fstest.MapFS{"000001_create_users.up.sql": {Data: mustReadMigration(t)}, "000002_failure.up.sql": {Data: []byte("CREATE TABLE rolled_back (id TEXT); INVALID SQL;")}}
	broken, e := migration.New(pool, "sqlite", bad)
	if e != nil {
		t.Fatal(e)
	}
	if e = broken.Up(ctx); e == nil {
		t.Fatal("bad SQL accepted")
	}
	var count int
	if e = pool.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name = 'rolled_back'").Scan(&count); e != nil || count != 0 {
		t.Fatal("migration DDL did not roll back", count, e)
	}
	var history int
	if e = pool.QueryRowContext(ctx, "SELECT count(*) FROM gonex_migrations").Scan(&history); e != nil || history != 1 {
		t.Fatal(history, e)
	}
}
func mustReadMigration(t *testing.T) []byte {
	t.Helper()
	b, e := migrations.Files.ReadFile("000001_create_users.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestPersistentHTTPAndAppDatabaseCleanup(t *testing.T) {
	d := sqliteDatabase(t)
	pool, _ := d.SQL()
	runner, _ := migration.New(pool, "sqlite", migrations.Files)
	if e := runner.Up(context.Background()); e != nil {
		t.Fatal(e)
	}
	a := newApp(t)
	if e := a.Register(moduleFunc(func(r *app.Registrar) error { return r.Manage(d) }), users.Module{Repository: users.NewGORMRepository(d)}); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Ada","email":"ada@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var u map[string]string
	if e = json.Unmarshal(w.Body.Bytes(), &u); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/users/"+u["id"], nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if e = a.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = a.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = pool.Ping(); e == nil {
		t.Fatal("app did not close database")
	}
}

func TestDatabaseFailuresAreRedacted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := orm.New(orm.Options{Dialect: "unsupported", DSN: "secret"}); e == nil {
		t.Fatal("bad dialect accepted")
	}
	if _, e := orm.Open(ctx, orm.Options{Dialect: "postgres", DSN: "postgres://user:supersecret@127.0.0.1:1/db?connect_timeout=1"}); e == nil || strings.Contains(e.Error(), "supersecret") {
		t.Fatal("connection failure not redacted", e)
	}
	if _, e := orm.Open(ctx, orm.Options{Dialect: "sqlite", DSN: filepath.Join(t.TempDir(), "missing", "database.db")}); e == nil {
		t.Fatal("bad path accepted")
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, e := orm.Open(cancelled, orm.Options{Dialect: "sqlite", DSN: ":memory:"}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("GONEX_TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("GONEX_REQUIRE_POSTGRES") == "1" {
			t.Fatal("required PostgreSQL DSN missing")
		}
		t.Skip("set GONEX_TEST_POSTGRES_DSN to an EMPTY disposable PostgreSQL database; SQLite integration runs unconditionally")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	d, e := orm.Open(ctx, orm.Options{Dialect: "postgres", DSN: dsn})
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close(context.Background())
	pool, _ := d.SQL()
	r, e := migration.New(pool, "postgres", migrations.Files)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Up(ctx); e != nil {
		t.Fatal(e)
	}
	if e = r.Up(ctx); e != nil {
		t.Fatal(e)
	}
	s := users.NewService(users.NewGORMRepository(d))
	u, e := s.Create(ctx, "Integration", time.Now().Format("150405.000000000")+"@example.com")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, u.ID); e != nil {
		t.Fatal(e)
	}
}
