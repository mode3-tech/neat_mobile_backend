package autorepaymentserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"neat_mobile_app_backend/internal/adapters/cba"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/modules/autorepayment"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/wallet"
	"neat_mobile_app_backend/providers/providus"
	"neat_mobile_app_backend/providers/push"
)

func NewRouter(cfg config.Config) (*gin.Engine, func(), error) {
	if strings.TrimSpace(cfg.DBUrl) == "" {
		return nil, nil, errors.New("DB_URL is required")
	}
	if strings.TrimSpace(cfg.ProvidusSecretKey) == "" {
		return nil, nil, errors.New("PROVIDUS_SECRET_KEY is required")
	}
	if strings.TrimSpace(cfg.LoanRepaymentAccountNumber) == "" {
		return nil, nil, errors.New("LOAN_REPAYMENT_ACCOUNT_NUMBER is required")
	}

	db, err := connectPostgresWithRetry(cfg.DBUrl, 5, time.Second)
	if err != nil {
		return nil, nil, err
	}

	if err := database.Migrate(db); err != nil {
		return nil, nil, err
	}

	autorepaymentRepo := autorepayment.NewRepository(db)
	walletRepo := wallet.NewRepository(db)
	notificationRepo := notification.NewRepository(db)
	expoSender := push.NewExpoClient(cfg.ExpoPushBaseURL, cfg.ExpoAccessToken)
	notificationService := notification.NewService(notificationRepo, expoSender, cfg.ExpoPushChannelID)
	providusClient := providus.NewProvidus(cfg.ProvidusSecretKey, cfg.ProvidusBaseURL)
	cbaClient := cba.NewProviderClient(cfg.CBAInternalURL, cfg.CBAInternalKey)

	settlementAccount := wallet.SettlementAccount{
		AccountNumber: cfg.LoanRepaymentAccountNumber,
		BankCode:      cfg.LoanRepaymentBankCode,
		AccountName:   cfg.LoanRepaymentAccountName,
	}

	autoRepaymentService := autorepayment.NewService(
		autorepaymentRepo,
		walletRepo,
		providusClient,
		cbaClient,
		notificationService,
		settlementAccount,
	)

	r := gin.New()
	r.Use(middleware.RequestContextLogger())
	r.Use(gin.Recovery())

	r.HEAD("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	c := cron.New(cron.WithLocation(time.UTC))

	var mu sync.Mutex
	var running bool

	c.AddFunc("0 6 * * *", func() {
		mu.Lock()
		if running {
			mu.Unlock()
			log.Print("auto-repayment sweep already running; skipping this trigger")
			return
		}
		running = true
		mu.Unlock()

		defer func() {
			mu.Lock()
			running = false
			mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := autoRepaymentService.ProcessDueRepayments(ctx); err != nil {
			log.Printf("auto-repayment sweep: %v", err)
		}
	})

	go c.Start()

	stopCron := func() { <-c.Stop().Done() }
	return r, stopCron, nil
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
