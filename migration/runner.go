// Package migration applies explicit ordered SQL files with database locking.
package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Migration struct {
	Version             int64
	Name, SQL, Checksum string
}
type Status struct {
	Version        int64
	Name, Checksum string
	Applied        bool
}
type Runner struct {
	db         *sql.DB
	dialect    string
	migrations []Migration
}

var filename = regexp.MustCompile(`^([0-9]{6,})_([a-z][a-z0-9_]*)\.up\.sql$`)

func New(db *sql.DB, dialect string, files fs.FS) (*Runner, error) {
	if db == nil || files == nil || dialect != "sqlite" && dialect != "postgres" {
		return nil, fmt.Errorf("migration needs database, supported dialect and files")
	}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	r := &Runner{db: db, dialect: dialect}
	seen := map[int64]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		match := filename.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %s", entry.Name())
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || version < 1 || seen[version] {
			return nil, fmt.Errorf("invalid or duplicate migration version %s", match[1])
		}
		seen[version] = true
		data, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(data)) == "" {
			return nil, fmt.Errorf("empty migration %s", entry.Name())
		}
		sum := sha256.Sum256(data)
		r.migrations = append(r.migrations, Migration{Version: version, Name: entry.Name(), SQL: string(data), Checksum: fmt.Sprintf("%x", sum)})
	}
	sort.Slice(r.migrations, func(i, j int) bool { return r.migrations[i].Version < r.migrations[j].Version })
	return r, nil
}

func (r *Runner) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("acquire migration transaction: %w", err)
	}
	defer tx.Rollback()
	if r.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(707177809)"); err != nil {
			return fmt.Errorf("acquire migration lock: %w", err)
		}
	}
	// SQLite's driver maps serializable to BEGIN IMMEDIATE, taking a cross-process write lock.
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS gonex_migrations (version BIGINT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("initialize migration history: %w", err)
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Runner) history(ctx context.Context, tx *sql.Tx) (map[int64]Status, error) {
	rows, err := tx.QueryContext(ctx, "SELECT version,name,checksum FROM gonex_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := map[int64]Status{}
	for rows.Next() {
		var s Status
		if err = rows.Scan(&s.Version, &s.Name, &s.Checksum); err != nil {
			return nil, err
		}
		s.Applied = true
		history[s.Version] = s
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	known := map[int64]bool{}
	pending := false
	for _, m := range r.migrations {
		known[m.Version] = true
		if old, ok := history[m.Version]; ok {
			if pending {
				return nil, fmt.Errorf("out-of-order applied migration %d", m.Version)
			}
			if old.Name != m.Name || old.Checksum != m.Checksum {
				return nil, fmt.Errorf("migration %d checksum or filename changed", m.Version)
			}
		} else {
			pending = true
		}
	}
	for v := range history {
		if !known[v] {
			return nil, fmt.Errorf("applied migration %d is missing from source", v)
		}
	}
	return history, nil
}

func (r *Runner) Up(ctx context.Context) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		history, err := r.history(ctx, tx)
		if err != nil {
			return err
		}
		for _, m := range r.migrations {
			if _, ok := history[m.Version]; ok {
				continue
			}
			if _, err = tx.ExecContext(ctx, m.SQL); err != nil {
				return fmt.Errorf("apply migration %d failed", m.Version)
			}
			query := "INSERT INTO gonex_migrations(version,name,checksum,applied_at) VALUES (?,?,?,?)"
			if r.dialect == "postgres" {
				query = "INSERT INTO gonex_migrations(version,name,checksum,applied_at) VALUES ($1,$2,$3,$4)"
			}
			if _, err = tx.ExecContext(ctx, query, m.Version, m.Name, m.Checksum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("record migration %d failed", m.Version)
			}
		}
		return nil
	})
}

func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	result := []Status{}
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		history, err := r.history(ctx, tx)
		if err != nil {
			return err
		}
		for _, m := range r.migrations {
			s := Status{Version: m.Version, Name: m.Name, Checksum: m.Checksum}
			if old, ok := history[m.Version]; ok {
				s = old
			}
			result = append(result, s)
		}
		return nil
	})
	return result, err
}
