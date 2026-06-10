package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthGuard(signer AccessTokenSigner, checker SessionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
		parts := strings.Fields(authHeader)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			log.Println("auth guard: missing or invalid authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" || !signer.ValidAccessToken(token) {
			log.Println("auth guard: invalid access token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}

		sub, sid, err := signer.ExtractAccessTokenIdentifiers(token)
		if err != nil || strings.TrimSpace(sub) == "" || strings.TrimSpace(sid) == "" {
			log.Printf("auth guard: failed to extract token identifiers: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}

		if checker != nil {
			ok, err := checker.IsSessionActive(c.Request.Context(), sid, sub, deviceID)
			if err != nil || !ok {
				log.Printf("auth guard: session not active or checker error: %v", err)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session is not active"})
				return
			}
		}

		c.Set(UserIDContextKey, sub)
		c.Set(SessionIDContextKey, sid)
		log.Printf("auth guard: authenticated user=%s session=%s device=%s", sub, sid, deviceID)
		c.Next()
	}
}
