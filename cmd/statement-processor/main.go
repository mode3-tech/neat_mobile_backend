package main

import (
	"context"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/statementprocessor"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func run(ctx context.Context) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	stopCron, err := statementprocessor.Run(cfg)
	if err != nil {
		return err
	}

	log.Print("statement processor started")
	<-ctx.Done()

	log.Print("shutting down statement processor...")
	stopCron()
	return nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found (using system environment)")
	}

	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
