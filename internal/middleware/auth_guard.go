package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthGuard(signer AccessTokenSigner, checker SessionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(authHeader)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" || !signer.ValidAccessToken(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}

		sub, sid, err := signer.ExtractAccessTokenIdentifiers(token)
		if err != nil || strings.TrimSpace(sub) == "" || strings.TrimSpace(sid) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}

		if checker != nil {
			ok, err := checker.IsSessionActive(c.Request.Context(), sid, sub)
			if err != nil || !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session is not active"})
				return
			}
		}

		c.Set(UserIDContextKey, sub)
		c.Set(SessionIDContextKey, sid)
		c.Next()
	}
}
