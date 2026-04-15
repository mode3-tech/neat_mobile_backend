package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/adapters/cba"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/modules/account"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/loanproduct"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/reporting"
	"neat_mobile_app_backend/modules/transaction"
	"neat_mobile_app_backend/modules/wallet"
	"neat_mobile_app_backend/providers/bvn/prembly"
	"neat_mobile_app_backend/providers/bvn/tendar"
	"neat_mobile_app_backend/providers/email"
	"neat_mobile_app_backend/providers/jwt"
	"neat_mobile_app_backend/providers/nin"
	"neat_mobile_app_backend/providers/providus"
	"neat_mobile_app_backend/providers/push"
	s3bucket "neat_mobile_app_backend/providers/s3_bucket"
	"neat_mobile_app_backend/providers/sms"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config) (*gin.Engine, func(), error) {
	db, err := connectPostgresWithRetry(cfg.DBUrl, 5, time.Second)
	if err != nil {
		return nil, nil, err
	}

	if cfg.RunMigrations {
		if err := database.Migrate(db); err != nil {
			return nil, nil, err
		}
	}

	r := gin.New()
	r.Use(middleware.RequestContextLogger())
	r.Use(gin.Recovery())
	r.StaticFile("/openapi/doc.json", "./docs/swagger.json")
	r.StaticFile("/openapi/doc.yaml", "./docs/swagger.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi/doc.json")))

	api := r.Group("/api")
	apiV1 := api.Group("/v1")
	internalV1 := r.Group("/internal/v1")

	r.HEAD("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if cfg.JWTSecret == "" {
		return nil, nil, errors.New("jwt secret can't be empty")
	}

	s3bucketConfig := s3bucket.BackblazeConfig{
		KeyID:      cfg.B2KeyID,
		AppKey:     cfg.B2AppKey,
		BucketName: cfg.B2StatementBucketName,
	}

	s3bucketClient, err := s3bucket.NewBackblazeClient(context.Background(), s3bucketConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create S3 bucket client: %w", err)
	}

	smsApiKey := cfg.TermiiApiKey
	smsSenderID := cfg.TermiiSenderID

	smsSender := sms.NewSMSService(smsApiKey, smsSenderID)
	emailSender := email.NewService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	tokenSigner := jwt.NewSigner(cfg.JWTSecret)
	bvnProvider := tendar.NewTendar(cfg.TendarAPIKey)
	premblyProvider := prembly.NewPrembly(cfg.PremblyAPIKey)

	var cbaClient *cba.ProviderClient
	var providerSource auth.BVNProviderSource
	if cfg.CBAInternalURL != "" && cfg.CBAInternalKey != "" {
		cbaClient = cba.NewProviderClient(cfg.CBAInternalURL, cfg.CBAInternalKey)
		providerSource = cbaClient
	} else {
		log.Print("CBA provider source is not fully configured; defaulting BVN validation to Tendar-first fallback")
	}
	transactor := tx.NewTransactor(db)
	deviceRepo := device.NewDeviceRepository(db)

	authRepo := auth.NewRespository(db)
	verificationRepo := verification.NewVerification(db)
	ninProvider := nin.NewNIN(cfg.PremblyAPIKey)
	loginRateLimiter := middleware.NewLoginRateLimiter(middleware.LoginRateLimiterConfig{
		IPMaxAttempts:    cfg.LoginRateLimitIPMaxAttempts,
		EmailMaxAttempts: cfg.LoginRateLimitEmailMaxAttempts,
		Window:           time.Duration(cfg.LoginRateLimitWindowMinutes) * time.Minute,
		BlockDuration:    time.Duration(cfg.LoginRateLimitBlockMinutes) * time.Minute,
	})

	providusWalletService := providus.NewProvidus(cfg.ProvidusSecretKey, cfg.ProvidusBaseURL)

	otpRepo := otp.NewOTPRepository(db)
	otpManager := otp.NewOTPManager(otpRepo, verificationRepo, transactor, smsSender, emailSender, cfg.Pepper)
	otpHandler := otp.NewOTPHandler(otpManager)
	otp.RegisterRoutes(apiV1, otpHandler)

	cbaSyncSem := make(chan struct{}, 10)
	cbaWalletUpdateSem := make(chan struct{}, 10)
	authService := auth.NewService(authRepo, cbaClient, cbaClient, verificationRepo, transactor, deviceRepo, smsSender, cfg.Pepper, tokenSigner, bvnProvider, premblyProvider, ninProvider, providerSource, otpManager, providusWalletService, cbaSyncSem, cbaWalletUpdateSem)
	authHandler := auth.NewHandler(authService)
	authGuard := middleware.AuthGuard(tokenSigner, nil)
	auth.RegisterRoutes(apiV1, authHandler, authGuard, loginRateLimiter.Middleware())

	authService.ConfigureOTPManager(otpManager)

	c := cron.New(cron.WithLocation(time.UTC))

	var mu sync.Mutex
	var running bool

	var stmtMu sync.Mutex
	var stmtRunning bool

	c.AddFunc("@every 10m", func() {
		mu.Lock()
		if running {
			mu.Unlock()
			return
		}
		running = true
		mu.Unlock()

		defer func() {
			mu.Lock()
			running = false
			mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if err := authService.SyncPendingCBACustomers(ctx); err != nil {
			log.Printf("cba sync sweep: %v", err)
		}
	})

	go func() {
		c.Start()
	}()

	stopCron := func() {
		<-c.Stop().Done()
	}

	loanRepo := loanproduct.NewRepository(db)
	loanService := loanproduct.NewService(loanRepo, cbaClient, cbaClient)
	loanHandler := loanproduct.NewHandler(loanService)
	loanproduct.RegisterRoutes(apiV1, loanHandler, authGuard)

	walletRepo := wallet.NewRepository(db)
	walletService := wallet.NewService(walletRepo, providusWalletService)
	walletHandler := wallet.NewHandler(walletService)
	wallet.RegisterRoutes(apiV1, walletHandler, authGuard)

	transactionRepo := transaction.NewRepository(db)
	transactionService := transaction.NewServie(transactionRepo)
	transactionHandler := transaction.NewHandler(transactionService)
	transaction.RegisterRoutes(apiV1, transactionHandler, authGuard)

	webhooksGroup := r.Group("/webhooks")
	if strings.TrimSpace(cfg.ProvidusWebhookSecret) == "" {
		log.Print("Providus webhook secret is not configured; credit webhook will reject all requests")
	}
	wallet.RegisterWebhookRoutes(webhooksGroup, walletHandler, middleware.ProvidusWebhookAuth(cfg.ProvidusWebhookSecret))

	expoSender := push.NewExpoClient(cfg.ExpoPushBaseURL, cfg.ExpoAccessToken)
	notificationRepo := notification.NewRepository(db)
	notificationService := notification.NewService(notificationRepo, expoSender, cfg.ExpoPushChannelID)

	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo, loanService, s3bucketClient, notificationService)
	accountHandler := account.NewHandler(accountService)
	account.RegisterRoutes(apiV1, accountHandler, authGuard)

	c.AddFunc("@every 30s", func() {
		stmtMu.Lock()
		if stmtRunning {
			stmtMu.Unlock()
			return
		}
		stmtRunning = true
		stmtMu.Unlock()

		defer func() {
			stmtMu.Lock()
			stmtRunning = false
			stmtMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		accountService.ProcessPendingStatementJobs(ctx)
	})

	internalLoanRepo := loanproduct.NewInternalRepository(db)
	internalLoanService := loanproduct.NewInternalService(internalLoanRepo)
	internalLoanHandler := loanproduct.NewInternalHandler(internalLoanService)
	internalAuth := middleware.InternalHMACAuth(cfg.CBAWebhookSecret)
	if strings.TrimSpace(cfg.CBAWebhookSecret) == "" {
		log.Print("CBA webhook secret is not configured; internal callback endpoints will reject requests")
	}
	loanproduct.RegisterInternalRoutes(internalV1, internalLoanHandler, internalAuth)

	reportingRepo := reporting.NewRepository(db)
	reportingService := reporting.NewService(reportingRepo)
	reportingHandler := reporting.NewHandler(reportingService)
	reporting.RegisterInternalRoutes(internalV1, reportingHandler, internalAuth)

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
