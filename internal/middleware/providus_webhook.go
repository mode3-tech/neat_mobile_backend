package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProvidusWebhookAuth verifies the X-Auth-Signature header Providus attaches
// to every credit notification. The value must match your configured webhook
// secret exactly. Confirm the exact header name with Providus documentation
// for your API version.
func ProvidusWebhookAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(secret) == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "webhook auth not configured"})
			return
		}

		incoming := strings.TrimSpace(c.GetHeader("X-Auth-Signature"))
		if incoming == "" || incoming != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
