package server

import (
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/adapters/cba"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/loanproduct"
	"neat_mobile_app_backend/providers/bvn/prembly"
	"neat_mobile_app_backend/providers/bvn/tendar"
	"neat_mobile_app_backend/providers/email"
	"neat_mobile_app_backend/providers/jwt"
	"neat_mobile_app_backend/providers/nin"
	"neat_mobile_app_backend/providers/sms"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config) (*gin.Engine, error) {
	db, err := connectPostgresWithRetry(cfg.DBUrl, 5, time.Second)
	if err != nil {
		return nil, err
	}

	if err := database.Migrate(db); err != nil {
		return nil, err
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

	if cfg.JWTSecret == "" {
		return nil, errors.New("jwt secret can't be empty")
	}
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

	authService := auth.NewAuthService(authRepo, verificationRepo, transactor, deviceRepo, tokenSigner, bvnProvider, premblyProvider, ninProvider, providerSource)
	authHandler := auth.NewAuthHandler(authService)
	authGuard := middleware.AuthGuard(tokenSigner, nil)
	auth.RegisterRoutes(apiV1, authHandler, authGuard, loginRateLimiter.Middleware())

	smsApiKey := cfg.TermiiApiKey
	smsSenderID := cfg.TermiiSenderID

	smsSender := sms.NewSMSService(smsApiKey, smsSenderID)
	emailSender := email.NewService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)
	authService.ConfigureLoginOTP(smsSender, cfg.Pepper)

	otpRepo := otp.NewOTPRepository(db)
	otpService := otp.NewOTPService(*otpRepo, verificationRepo, transactor, smsSender, emailSender, cfg.Pepper)
	otpHandler := otp.NewOTPHandler(otpService)
	otp.RegisterRoutes(apiV1, otpHandler)

	loanRepo := loanproduct.NewRepository(db)
	loanService := loanproduct.NewService(loanRepo, cbaClient, cbaClient)
	loanHandler := loanproduct.NewHandler(loanService)
	loanproduct.RegisterRoutes(apiV1, loanHandler, authGuard)

	internalLoanRepo := loanproduct.NewInternalRepository(db)
	internalLoanService := loanproduct.NewInternalService(internalLoanRepo)
	internalLoanHandler := loanproduct.NewInternalHandler(internalLoanService)
	internalAuth := middleware.InternalHMACAuth(cfg.CBAWebhookSecret)
	if strings.TrimSpace(cfg.CBAWebhookSecret) == "" {
		log.Print("CBA webhook secret is not configured; internal callback endpoints will reject requests")
	}
	loanproduct.RegisterInternalRoutes(internalV1, internalLoanHandler, internalAuth)

	return r, nil
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
