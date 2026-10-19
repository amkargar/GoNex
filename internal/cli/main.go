package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	dir, err := os.Getwd()
	if err != nil {
		stop()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := Execute(ctx, os.Args[1:], dir, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
