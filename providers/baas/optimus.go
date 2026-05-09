package baas

import (
	"net/http"
	"time"
)

type Optimus struct {
	BaseURL string
	ApiKey  string
	Client  *http.Client
}

func NewOptimus(baseURL string, apiKey string) *Optimus {
	return &Optimus{BaseURL: baseURL, ApiKey: apiKey, Client: &http.Client{Timeout: time.Second * 15}}
}

func (o *Optimus) GenerateWallet()
