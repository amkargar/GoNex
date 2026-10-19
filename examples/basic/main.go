package main

import (
	"context"
	"github.com/amkargar/gonex"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/basic/modules/healthdemo"
	"github.com/amkargar/gonex/examples/basic/modules/users"
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
	if err = a.Register(users.Module{}, healthdemo.Module{}); err != nil {
		log.Fatal(err)
	}
	if err = a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
