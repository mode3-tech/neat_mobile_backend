package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
	"xpress/internal/config"
	"xpress/internal/server"

	"github.com/joho/godotenv"
)

func run(ctx context.Context) error {
	errChan := make(chan error, 1)

	cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	srv, err := server.New(cfg)

	if err != nil {
		return err
	}

	go func() {
		log.Printf("Listening on port %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	var wg sync.WaitGroup

	wg.Go(func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			errChan <- err
		}
	})

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println(".env file not found (using system environment)")
	}
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
