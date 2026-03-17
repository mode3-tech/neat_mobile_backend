package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func InternalHMACAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(secret) == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal auth not configured"})
			return
		}

		ts := strings.TrimSpace(c.GetHeader("X-Timestamp"))
		sig := strings.TrimSpace(c.GetHeader("X-Signature"))
		if ts == "" || sig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing internal auth headers"})
			return
		}

		t, err := time.Parse(time.RFC3339, ts)
		if err != nil || time.Since(t) > 5*time.Minute || time.Until(t) > 5*time.Minute {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "stale timestamp"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid request body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		bodyHash := sha256.Sum256(body)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(c.Request.Method))
		mac.Write([]byte("\n"))
		mac.Write([]byte(c.Request.URL.Path))
		mac.Write([]byte("\n"))
		mac.Write([]byte(ts))
		mac.Write([]byte("\n"))
		mac.Write([]byte(hex.EncodeToString(bodyHash[:])))

		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(strings.ToLower(sig)), []byte(expected)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}

		c.Next()
	}
}
