package autorepaymentserver

import (
	"neat_mobile_app_backend/internal/config"
	"net/http"
	"time"
)

func New(cfg config.Config) (*http.Server, func(), error) {
	router, stopCron, err := NewRouter(cfg)
	if err != nil {
		return nil, nil, err
	}

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}, stopCron, nil
}
