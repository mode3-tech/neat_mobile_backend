package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProvidusWebhookAuth verifies the X-Auth-Signature header that Providus
// attaches to every credit notification. The header value is an HMAC-SHA512
// signature of the raw request body, signed with the configured webhook secret.
func ProvidusWebhookAuth(secret string) gin.HandlerFunc {
	secretTrimmed := strings.TrimSpace(secret)
	return func(c *gin.Context) {
		if secretTrimmed == "" {
			log.Print("Providus webhook secret is not configured; credit webhook will reject all requests")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "webhook auth not configured"})
			return
		}

		incoming := strings.TrimSpace(c.GetHeader("x-xpresswallet-signature"))
		if incoming == "" {
			log.Print("providus webhook: missing x-xpresswallet-signature header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("providus webhook: failed to read body: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to read body"})
			return
		}
		// Restore the body so downstream handlers can read it.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		mac := hmac.New(sha512.New, []byte(secretTrimmed))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expected), []byte(incoming)) {
			log.Printf("providus webhook: invalid signature expected=%s incoming=%s bodyLen=%d", expected, incoming, len(body))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
