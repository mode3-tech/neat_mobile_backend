package server

import (
	"net/http"
	"time"
	"xpress/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.Config) (*http.Server, error) {

	router, err := NewRouter(cfg)
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s, nil
}
