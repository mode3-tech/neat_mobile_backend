package notificationserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/providers/jwt"
	"neat_mobile_app_backend/providers/push"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config) (*gin.Engine, func(), error) {
	db, err := connectPostgresWithRetry(cfg.DBUrl, 5, time.Second)
	if err != nil {
		return nil, nil, err
	}

	if err := database.Migrate(db); err != nil {
		return nil, nil, err
	}

	if cfg.JWTSecret == "" {
		return nil, nil, errors.New("jwt secret can't be empty")
	}

	r := gin.New()
	r.Use(middleware.RequestContextLogger())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	apiV1 := api.Group("/v1")
	internalV1 := r.Group("/internal/v1")

	tokenSigner := jwt.NewSigner(cfg.JWTSecret)
	authGuard := middleware.AuthGuard(tokenSigner, nil)

	expoSender := push.NewExpoClient(cfg.ExpoPushBaseURL, cfg.ExpoAccessToken)
	notificationRepo := notification.NewRepository(db)
	notificationService := notification.NewService(notificationRepo, expoSender, cfg.ExpoPushChannelID)
	notificationHandler := notification.NewHandler(notificationService)

	notification.RegisterRoutes(apiV1, notificationHandler, authGuard)

	internalAuth := middleware.InternalHMACAuth(cfg.NotificationInternalSecret)
	if strings.TrimSpace(cfg.NotificationInternalSecret) == "" {
		log.Print("notification internal secret is not configured; internal notification endpoints will reject requests")
	}
	notification.RegisterInternalRoutes(internalV1, notificationHandler, internalAuth)

	c := cron.New(cron.WithLocation(time.UTC))
	c.AddFunc("@every 5m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := notificationService.ProcessPendingReceipts(ctx); err != nil {
			log.Printf("receipt poll: %v", err)
		}
	})
	c.Start()

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
