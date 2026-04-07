package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func run(ctx context.Context) error {
	// cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	return nil

}
