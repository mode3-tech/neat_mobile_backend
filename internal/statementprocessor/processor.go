package statementprocessor

import (
	"context"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/adapters/cba"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/modules/account"
	"neat_mobile_app_backend/modules/loanproduct"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/providers/push"
	s3bucket "neat_mobile_app_backend/providers/s3_bucket"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

func Run(cfg config.Config) (stop func(), err error) {
	db, err := connectPostgresWithRetry(cfg.DBUrl, 5, time.Second)
	if err != nil {
		return nil, err
	}

	b2Client, err := s3bucket.NewBackblazeClient(context.Background(), s3bucket.BackblazeConfig{
		KeyID:      cfg.B2KeyID,
		AppKey:     cfg.B2AppKey,
		BucketName: cfg.B2StatementBucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create B2 client: %w", err)
	}

	expoSender := push.NewExpoClient(cfg.ExpoPushBaseURL, cfg.ExpoAccessToken)
	notificationRepo := notification.NewRepository(db)
	notifier := notification.NewService(notificationRepo, expoSender, cfg.ExpoPushChannelID)

	var cbaClient *cba.ProviderClient
	if cfg.CBAInternalURL != "" && cfg.CBAInternalKey != "" {
		cbaClient = cba.NewProviderClient(cfg.CBAInternalURL, cfg.CBAInternalKey)
	} else {
		log.Print("statement processor: CBA not configured, loan balances will be unavailable in statements")
	}

	loanRepo := loanproduct.NewRepository(db)
	loanService := loanproduct.NewService(loanRepo, cbaClient, cbaClient)

	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo, loanService, b2Client, notifier)

	c := cron.New(cron.WithLocation(time.UTC))

	var mu sync.Mutex
	var running bool

	c.AddFunc("@every 30s", func() {
		mu.Lock()
		if running {
			mu.Unlock()
			log.Print("statement processor: previous run still in progress, skipping")
			return
		}
		running = true
		mu.Unlock()

		defer func() {
			mu.Lock()
			running = false
			mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		accountService.ProcessPendingStatementJobs(ctx)
	})

	go c.Start()

	return func() { <-c.Stop().Done() }, nil
}

func connectPostgresWithRetry(dsn string, attempts int, baseDelay time.Duration) (*gorm.DB, error) {
	if attempts <= 0 {
		attempts = 1
	}
	if baseDelay <= 0 {
		baseDelay = time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		db, err := database.NewPostgres(dsn)
		if err == nil {
			return db, nil
		}
		lastErr = err
		if attempt == attempts {
			break
		}
		delay := baseDelay * time.Duration(attempt)
		log.Printf("database connection attempt %d/%d failed: %v (retrying in %s)", attempt, attempts, err, delay)
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("database connection failed after %d attempts: %w", attempts, lastErr)
}
