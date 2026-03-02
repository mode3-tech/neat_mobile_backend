package server

import (
	"log"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/providers/bvn/prembly"
	"neat_mobile_app_backend/providers/bvn/tendar"
	"neat_mobile_app_backend/providers/email"
	"neat_mobile_app_backend/providers/jwt"
	"neat_mobile_app_backend/providers/sms"

	"github.com/gin-gonic/gin"
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

	api := r.Group("/api")
	apiV1 := api.Group("/v1")

	tokenSigner := jwt.NewSigner(cfg.JWTSecret)
	bvnProvider := tendar.NewTendar(cfg.TendarAPIKey)
	premblyProvider := prembly.NewPrembly(cfg.PremblyAPIKey)

	authRepo := auth.NewRespository(db)
	authService := auth.NewService(authRepo, tokenSigner, bvnProvider, premblyProvider)
	authHandler := auth.NewHandler(authService)
	auth.RegisterRoutes(apiV1, authHandler)

	smsApiKey := cfg.SMSLiveApiKey
	smsSenderID := cfg.SMSLiveSenderID

	smsSender := sms.NewSMSService(smsApiKey, smsSenderID)
	emailSender := email.NewService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	log.Printf("Email service configured with host: %s, port: %s, user: %s and password: %s", cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	otpRepo := otp.NewOTPRepository(db)
	otpService := otp.NewOTPService(*otpRepo, smsSender, emailSender, tokenSigner, cfg.Pepper)
	otpHandler := otp.NewOTPHandler(otpService)
	otp.RegisterRoutes(apiV1, otpHandler)

	return r, nil
}
