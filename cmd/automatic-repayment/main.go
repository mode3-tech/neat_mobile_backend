package main

import (
	"context"
	"neat_mobile_app_backend/internal/config"
	"os"
	"os/signal"
	"syscall"
)

func run(ctx context.Context) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

}
