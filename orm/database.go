// Package orm owns GORM connections without exposing them to business services.
package orm

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"sync"
	"time"
)

type Options struct {
	Dialect                  string
	DSN                      string
	DSNEnv                   string
	MaxOpen, MaxIdle         int
	MaxLifetime, MaxIdleTime time.Duration
}

type Database struct {
	mu      sync.Mutex
	options Options
	db      *gorm.DB
	pool    *sql.DB
	closed  bool
}

func New(o Options) (*Database, error) {
	if o.Dialect != "sqlite" && o.Dialect != "postgres" {
		return nil, fmt.Errorf("database dialect must be sqlite or postgres")
	}
	if o.DSN != "" && o.DSNEnv != "" {
		return nil, fmt.Errorf("choose DSN or DSNEnv")
	}
	if o.DSNEnv != "" {
		o.DSN = os.Getenv(o.DSNEnv)
	}
	if o.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}
	if o.MaxOpen < 0 || o.MaxIdle < 0 || o.MaxLifetime < 0 || o.MaxIdleTime < 0 {
		return nil, fmt.Errorf("invalid database pool settings")
	}
	if o.MaxOpen == 0 {
		o.MaxOpen = 10
	}
	if o.MaxIdle == 0 {
		o.MaxIdle = 2
	}
	if o.MaxIdle > o.MaxOpen {
		o.MaxIdle = o.MaxOpen
	}
	if o.MaxLifetime == 0 {
		o.MaxLifetime = 30 * time.Minute
	}
	return &Database{options: o}, nil
}

func Open(ctx context.Context, o Options) (*Database, error) {
	d, e := New(o)
	if e != nil {
		return nil, e
	}
	if e = d.Start(ctx); e != nil {
		return nil, e
	}
	return d, nil
}

func (d *Database) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return fmt.Errorf("database is closed")
	}
	if d.db != nil {
		if err := d.pool.PingContext(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("database connection failed")
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var dialector gorm.Dialector
	if d.options.Dialect == "sqlite" {
		dialector = gormlite.Open(d.options.DSN)
	} else {
		dialector = postgres.Open(d.options.DSN)
	}
	db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true, TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("database configuration failed")
	}
	pool, err := db.DB()
	if err != nil {
		return fmt.Errorf("database pool unavailable")
	}
	pool.SetMaxOpenConns(d.options.MaxOpen)
	pool.SetMaxIdleConns(d.options.MaxIdle)
	pool.SetConnMaxLifetime(d.options.MaxLifetime)
	pool.SetConnMaxIdleTime(d.options.MaxIdleTime)
	if err = pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("database connection failed")
	}
	d.db = db
	d.pool = pool
	return nil
}

func (d *Database) DB() (*gorm.DB, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db == nil || d.closed {
		return nil, fmt.Errorf("database is not running")
	}
	return d.db, nil
}
func (d *Database) SQL() (*sql.DB, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pool == nil || d.closed {
		return nil, fmt.Errorf("database is not running")
	}
	return d.pool, nil
}
func (d *Database) Close(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	if d.pool != nil {
		return d.pool.Close()
	}
	return nil
}

// Transaction belongs at an adapter boundary. Services can depend on a local
// callback interface implemented by an adapter, without importing GORM.
func Transaction(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	if db == nil || fn == nil {
		return fmt.Errorf("transaction needs a database and callback")
	}
	return db.WithContext(ctx).Transaction(fn)
}
