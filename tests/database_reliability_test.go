package tests

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/orm"
)

type databaseFixture struct {
	db           *orm.Database
	pool         *sql.DB
	dialect, dsn string
}

func reliableDatabase(t *testing.T, dialect string) databaseFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "reliability.db")) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if dialect == "postgres" {
		dsn = os.Getenv("GONEX_TEST_POSTGRES_DSN")
		if dsn == "" {
			if os.Getenv("GONEX_REQUIRE_POSTGRES") == "1" {
				t.Fatal("required disposable PostgreSQL DSN missing")
			}
			t.Skip("PostgreSQL integration requires GONEX_TEST_POSTGRES_DSN")
		}
		admin, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSN: dsn})
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("gonex_%d", time.Now().UnixNano())
		pool, _ := admin.SQL()
		if _, err = pool.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			c, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_, err := pool.ExecContext(c, `DROP SCHEMA "`+schema+`" CASCADE`)
			if err != nil {
				t.Error(err)
			}
			admin.Close(c)
		})
		u, err := url.Parse(dsn)
		if err != nil || u.Scheme == "" {
			dsn += " search_path=" + schema
		} else {
			q := u.Query()
			q.Set("search_path", schema)
			u.RawQuery = q.Encode()
			dsn = u.String()
		}
	}
	db, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSN: dsn, MaxOpen: 2, MaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.SQL()
	t.Cleanup(func() { db.Close(context.Background()) })
	return databaseFixture{db, pool, dialect, dsn}
}

func TestDatabaseReliability(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := reliableDatabase(t, dialect)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			statements := []string{
				"CREATE TABLE parents (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE)",
				"CREATE TABLE children (id TEXT PRIMARY KEY, parent_id TEXT NOT NULL REFERENCES parents(id))",
				"INSERT INTO parents VALUES ('1','Ada')",
				"INSERT INTO children VALUES ('1','1')",
			}
			for _, q := range statements {
				if _, err := f.pool.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}
			for _, q := range []string{"INSERT INTO parents VALUES ('2','Ada')", "INSERT INTO children VALUES ('2','missing')"} {
				if _, err := f.pool.ExecContext(ctx, q); err == nil {
					t.Fatal("constraint ignored", q)
				}
			}
			tx, err := f.pool.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO parents VALUES ('rollback','Rollback')"); err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			var n int
			if err = f.pool.QueryRowContext(ctx, "SELECT count(*) FROM parents WHERE id='rollback'").Scan(&n); err != nil || n != 0 {
				t.Fatal("transaction did not roll back", n, err)
			}
			if _, err = f.pool.ExecContext(ctx, "UPDATE parents SET name='Grace' WHERE id='1'"); err != nil {
				t.Fatal(err)
			}
			var name string
			if err = f.pool.QueryRowContext(ctx, "SELECT name FROM parents WHERE id='1'").Scan(&name); err != nil || name != "Grace" {
				t.Fatal(name, err)
			}
			if _, err = f.pool.ExecContext(ctx, "DELETE FROM children WHERE id='1'"); err != nil {
				t.Fatal(err)
			}
			if _, err = f.pool.ExecContext(ctx, "DELETE FROM parents WHERE id='1'"); err != nil {
				t.Fatal(err)
			}
			cancelled, stop := context.WithCancel(ctx)
			stop()
			if _, err = f.pool.ExecContext(cancelled, "SELECT 1"); err == nil {
				t.Fatal("cancelled query succeeded")
			}
			if dialect == "postgres" {
				bounded, stop := context.WithTimeout(ctx, 100*time.Millisecond)
				defer stop()
				start := time.Now()
				if _, err = f.pool.ExecContext(bounded, "SELECT pg_sleep(5)"); err == nil {
					t.Fatal("deadline ignored")
				}
				if time.Since(start) > 2*time.Second {
					t.Fatal("query cancellation exceeded budget")
				}
			}
		})
	}
}

func TestMigrationProcessesAndRecovery(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := reliableDatabase(t, dialect)
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			errs := make(chan string, 3)
			for i := 0; i < 3; i++ {
				wg.Go(func() {
					cmd := exec.CommandContext(ctx, executable, "-test.run=^TestMigrationProcessWorker$")
					cmd.Env = append(os.Environ(), "GONEX_MIGRATION_WORKER=1", "GONEX_WORKER_DIALECT="+dialect, "GONEX_WORKER_DSN="+f.dsn)
					out, err := cmd.CombinedOutput()
					if err != nil {
						errs <- fmt.Sprintf("%v: %s", err, out)
					}
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			if t.Failed() {
				return
			}
			var n int
			if err = f.pool.QueryRowContext(ctx, "SELECT count(*) FROM process_effect").Scan(&n); err != nil || n != 1 {
				t.Fatal("migration duplicated", n, err)
			}
			good := processMigrations()
			bad := processMigrations()
			bad["000002_fail.up.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE must_rollback(id TEXT); INVALID SQL;")}
			runner, err := migration.New(f.pool, dialect, bad)
			if err != nil {
				t.Fatal(err)
			}
			if err = runner.Up(ctx); err == nil {
				t.Fatal("broken migration accepted")
			}
			// A transactional DDL failure must not persist its table or history entry.
			if _, err = f.pool.ExecContext(ctx, "SELECT * FROM must_rollback"); err == nil {
				t.Fatal("partial DDL survived rollback")
			}
			if err = f.pool.QueryRowContext(ctx, "SELECT count(*) FROM gonex_migrations").Scan(&n); err != nil || n != 1 {
				t.Fatal(n, err)
			}
			good["000002_fixed.up.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE recovered(id TEXT);")}
			runner, _ = migration.New(f.pool, dialect, good)
			if err = runner.Up(ctx); err != nil {
				t.Fatal("forward recovery failed", err)
			}
			good["000001_process.up.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
			runner, _ = migration.New(f.pool, dialect, good)
			if err = runner.Up(ctx); err == nil || !strings.Contains(err.Error(), "checksum") {
				t.Fatal("changed applied history accepted", err)
			}
		})
	}
}
func processMigrations() fstest.MapFS {
	return fstest.MapFS{"000001_process.up.sql": {Data: []byte("CREATE TABLE process_effect(id TEXT PRIMARY KEY); INSERT INTO process_effect VALUES ('once');")}}
}
func TestMigrationProcessWorker(t *testing.T) {
	if os.Getenv("GONEX_MIGRATION_WORKER") != "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dialect := os.Getenv("GONEX_WORKER_DIALECT")
	db, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSN: os.Getenv("GONEX_WORKER_DSN")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	pool, _ := db.SQL()
	r, err := migration.New(pool, dialect, processMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Up(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresOutageAndRecovery(t *testing.T) {
	f := reliableDatabase(t, "postgres")
	u, err := url.Parse(f.dsn)
	if err != nil || u.Host == "" {
		t.Fatal("outage test requires a URL DSN")
	}
	target := u.Host
	if !strings.Contains(target, ":") {
		target = net.JoinHostPort(target, "5432")
	}
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	available := true
	var connections []net.Conn
	var wg sync.WaitGroup
	stop := func() {
		mu.Lock()
		available = false
		for _, c := range connections {
			c.Close()
		}
		mu.Unlock()
	}
	t.Cleanup(func() { proxy.Close(); stop(); wg.Wait() })
	wg.Go(func() {
		for {
			client, e := proxy.Accept()
			if e != nil {
				return
			}
			mu.Lock()
			ok := available
			connections = append(connections, client)
			mu.Unlock()
			if !ok {
				client.Close()
				continue
			}
			server, e := net.DialTimeout("tcp", target, time.Second)
			if e != nil {
				client.Close()
				continue
			}
			mu.Lock()
			connections = append(connections, server)
			mu.Unlock()
			wg.Go(func() {
				defer client.Close()
				defer server.Close()
				done := make(chan struct{})
				go func() { io.Copy(server, client); server.Close(); close(done) }()
				io.Copy(client, server)
				client.Close()
				<-done
			})
		}
	})
	u.Host = proxy.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := orm.Open(ctx, orm.Options{Dialect: "postgres", DSN: u.String(), MaxOpen: 1, MaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	stop()
	bounded, stopCheck := context.WithTimeout(ctx, 300*time.Millisecond)
	if db.Ready(bounded) == nil {
		t.Fatal("outage reported ready")
	}
	stopCheck()
	mu.Lock()
	available = true
	mu.Unlock()
	deadline := time.Now().Add(4 * time.Second)
	for {
		check, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		err = db.Ready(check)
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pool failed to recover", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
