package server

import (
	"xpress/internal/config"
	"xpress/internal/database"
	"xpress/modules/auth"
	"xpress/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config) (*gin.Engine, error) {
	db, err := database.NewPostgres(cfg.DBUrl)

	if err != nil {
		return nil, err
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	api := r.Group("/api")
	apiV1 := api.Group("/v1")

	tokenSigner := jwt.NewSigner(cfg.JWTSecret)

	authRepo := auth.NewRespository(db)
	authService := auth.NewService(authRepo, tokenSigner)
	authHandler := auth.NewHandler(authService)
	auth.RegisterRoutes(apiV1, authHandler)

	return r, nil
}
