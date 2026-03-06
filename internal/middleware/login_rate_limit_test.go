package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLoginRateLimiter_BlocksByIPAfterFailedAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewLoginRateLimiter(LoginRateLimiterConfig{
		IPMaxAttempts:    3,
		EmailMaxAttempts: 10,
		Window:           time.Minute,
		BlockDuration:    time.Minute,
	})

	router := gin.New()
	router.POST("/api/v1/auth/login", limiter.Middleware(), func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	for i := 0; i < 3; i++ {
		resp := performLoginRequest(router, "1.1.1.1:1234", "user@example.com", "bad")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected status %d, got %d", i+1, http.StatusUnauthorized, resp.Code)
		}
	}

	resp := performLoginRequest(router, "1.1.1.1:1234", "user@example.com", "bad")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d after threshold, got %d", http.StatusTooManyRequests, resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if _, ok := body["retry_after_seconds"]; !ok {
		t.Fatal("expected retry_after_seconds in 429 response")
	}
}

func TestLoginRateLimiter_BlocksByEmailAcrossDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewLoginRateLimiter(LoginRateLimiterConfig{
		IPMaxAttempts:    100,
		EmailMaxAttempts: 2,
		Window:           time.Minute,
		BlockDuration:    time.Minute,
	})

	router := gin.New()
	router.POST("/api/v1/auth/login", limiter.Middleware(), func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	resp := performLoginRequest(router, "1.1.1.1:1234", "target@example.com", "bad")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 1: expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}

	resp = performLoginRequest(router, "2.2.2.2:1234", "target@example.com", "bad")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 2: expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}

	resp = performLoginRequest(router, "3.3.3.3:1234", "target@example.com", "bad")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 3: expected status %d, got %d", http.StatusTooManyRequests, resp.Code)
	}
}

func TestLoginRateLimiter_SuccessfulLoginResetsCounters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewLoginRateLimiter(LoginRateLimiterConfig{
		IPMaxAttempts:    2,
		EmailMaxAttempts: 2,
		Window:           time.Minute,
		BlockDuration:    time.Minute,
	})

	router := gin.New()
	router.POST("/api/v1/auth/login", limiter.Middleware(), func(c *gin.Context) {
		var payload struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		if payload.Password == "good" {
			c.Status(http.StatusOK)
			return
		}

		c.Status(http.StatusUnauthorized)
	})

	resp := performLoginRequest(router, "9.9.9.9:1234", "reset@example.com", "bad")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 1: expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}

	resp = performLoginRequest(router, "9.9.9.9:1234", "reset@example.com", "good")
	if resp.Code != http.StatusOK {
		t.Fatalf("attempt 2: expected status %d, got %d", http.StatusOK, resp.Code)
	}

	resp = performLoginRequest(router, "9.9.9.9:1234", "reset@example.com", "bad")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 3: expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}

	resp = performLoginRequest(router, "9.9.9.9:1234", "reset@example.com", "bad")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 4: expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}

	resp = performLoginRequest(router, "9.9.9.9:1234", "reset@example.com", "bad")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 5: expected status %d, got %d", http.StatusTooManyRequests, resp.Code)
	}
}

func performLoginRequest(router *gin.Engine, remoteAddr, email, password string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}
