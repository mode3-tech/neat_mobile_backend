package main

import (
	"context"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/notificationserver"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func run(ctx context.Context) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, stopCron, err := notificationserver.New(cfg)
	if err != nil {
		return err
	}

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Notification service listening on port %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopCron()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
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
