package cba

import (
	"context"
	"encoding/json"
	"fmt"
	"neat_mobile_app_backend/modules/auth"
	"net/http"
	"strings"
	"time"
)

type ProviderClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      *ttlCache
}

func NewProviderClient(baseURL, apiKey string) *ProviderClient {
	return &ProviderClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		cache:      newTTLCache(30 * time.Second),
	}
}

func (c *ProviderClient) GetCurrentProvider(ctx context.Context) (auth.Provider, error) {
	if c.baseURL == "" {
		return "", fmt.Errorf("cba base url is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("cba internal key is not configured")
	}

	// TTL cache to avoid calling CBA on every request
	return c.cache.Get(func() (auth.Provider, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/bvnValidationPreference", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-Internal-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("cba returned status %d", resp.StatusCode)
		}

		var out struct {
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", err
		}

		p := auth.Provider(strings.ToLower(strings.TrimSpace(out.Provider)))
		switch p {
		case auth.ProviderTendar, auth.ProviderPrembly:
			return p, nil
		default:
			// safe default
			return auth.ProviderTendar, nil
		}
	})
}

type ttlCache struct {
	ttl       time.Duration
	value     auth.Provider
	expiresAt time.Time
	ok        bool
	mu        chan struct{}
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, mu: make(chan struct{}, 1)}
}

func (c *ttlCache) Get(fetch func() (auth.Provider, error)) (auth.Provider, error) {
	c.mu <- struct{}{}
	defer func() { <-c.mu }()

	now := time.Now()
	if c.ok && now.Before(c.expiresAt) {
		return c.value, nil
	}

	v, err := fetch()
	if err != nil {
		// fallback to last good value if we have one
		if c.ok {
			return c.value, nil
		}
		return "", err
	}

	c.value = v
	c.ok = true
	c.expiresAt = now.Add(c.ttl)
	return v, nil
}
