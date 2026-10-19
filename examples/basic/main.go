package main

import (
	"context"
	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/basic/migrations"
	"github.com/amkargar/gonex/examples/basic/modules/healthdemo"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/orm"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := gonex.New(config.Config{Address: os.Getenv("HTTP_ADDRESS")})
	if err != nil {
		log.Fatal(err)
	}
	var repo users.Repository
	if os.Getenv("DATABASE_URL") != "" {
		dialect := os.Getenv("DATABASE_DIALECT")
		if dialect == "" {
			dialect = "sqlite"
		}
		db, e := orm.New(orm.Options{Dialect: dialect, DSNEnv: "DATABASE_URL"})
		if e != nil {
			log.Fatal(e)
		}
		if len(os.Args) > 1 && os.Args[1] == "migrate" {
			if e = db.Start(ctx); e != nil {
				log.Fatal(e)
			}
			defer db.Close(context.Background())
			pool, e := db.SQL()
			if e != nil {
				log.Fatal(e)
			}
			runner, e := migration.New(pool, dialect, migrations.Files)
			if e != nil {
				log.Fatal(e)
			}
			if e = runner.Up(ctx); e != nil {
				log.Fatal(e)
			}
			return
		}
		if e = a.Register(databaseModule{db}); e != nil {
			log.Fatal(e)
		}
		repo = users.NewGORMRepository(db)
	}
	if err = a.Register(users.Module{Repository: repo}, healthdemo.Module{}); err != nil {
		log.Fatal(err)
	}
	if err = a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

type databaseModule struct{ db *orm.Database }

func (m databaseModule) Register(r *app.Registrar) error { return r.Manage(m.db) }
