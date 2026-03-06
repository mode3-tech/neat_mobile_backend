package server

import (
	"errors"
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
	"neat_mobile_app_backend/providers/bvn/prembly"
	"neat_mobile_app_backend/providers/bvn/tendar"
	"neat_mobile_app_backend/providers/email"
	"neat_mobile_app_backend/providers/jwt"
	"neat_mobile_app_backend/providers/nin"
	"neat_mobile_app_backend/providers/sms"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(cfg config.Config) (*gin.Engine, error) {
	db, err := database.NewPostgres(cfg.DBUrl)
	if err != nil {
		return nil, err
	}

	if err := database.Migrate(db); err != nil {
		return nil, err
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.StaticFile("/openapi/doc.json", "./docs/swagger.json")
	r.StaticFile("/openapi/doc.yaml", "./docs/swagger.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi/doc.json")))

	api := r.Group("/api")
	apiV1 := api.Group("/v1")

	if cfg.JWTSecret == "" {
		return nil, errors.New("jwt secret can't be empty")
	}
	tokenSigner := jwt.NewSigner(cfg.JWTSecret)
	bvnProvider := tendar.NewTendar(cfg.TendarAPIKey)
	premblyProvider := prembly.NewPrembly(cfg.PremblyAPIKey)

	var providerSource auth.BVNProviderSource
	if cfg.CBAInternalURL != "" && cfg.CBAInternalKey != "" {
		providerSource = cba.NewProviderClient(cfg.CBAInternalURL, cfg.CBAInternalKey)
	} else {
		log.Print("CBA provider source is not fully configured; BVN validation will fail until CBA_INTERNAL_URL and CBA_INTERNAL_KEY are set")
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
	auth.RegisterRoutes(apiV1, authHandler, loginRateLimiter.Middleware())

	smsApiKey := cfg.TermiiApiKey
	smsSenderID := cfg.TermiiSenderID

	smsSender := sms.NewSMSService(smsApiKey, smsSenderID)
	emailSender := email.NewService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	otpRepo := otp.NewOTPRepository(db)

	otpService := otp.NewOTPService(*otpRepo, verificationRepo, transactor, smsSender, emailSender, tokenSigner, cfg.Pepper)
	otpHandler := otp.NewOTPHandler(otpService)
	otp.RegisterRoutes(apiV1, otpHandler)

	return r, nil
}
