package server

import (
	"neat_mobile_app_backend/internal/config"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.Config) (*http.Server, func(), error) {

	router, stopCron, err := NewRouter(cfg)
	if err != nil {
		return nil, stopCron, err
	}

	s := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s, stopCron, nil
}
