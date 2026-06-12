package middleware

import (
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
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
			mapped := response.MapError(appErr.ErrMissingUserID)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" || !signer.ValidAccessToken(token) {
			log.Println("auth guard: invalid access token")
			mapped := response.MapError(appErr.ErrUnauthorized)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}

		sub, sid, err := signer.ExtractAccessTokenIdentifiers(token)
		if err != nil || strings.TrimSpace(sub) == "" || strings.TrimSpace(sid) == "" {
			log.Printf("auth guard: failed to extract token identifiers: %v", err)
			mapped := response.MapError(appErr.ErrUnauthorized)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}

		if checker != nil {
			ok, err := checker.IsSessionActive(c.Request.Context(), sid, sub, deviceID)
			if err != nil || !ok {
				log.Printf("auth guard: session not active or checker error: %v", err)
				mapped := response.MapError(appErr.ErrInvalidSession)
				c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
					Status: "error",
					Error:  &mapped.Error,
				})
				return
			}
		}

		c.Set(UserIDContextKey, sub)
		c.Set(SessionIDContextKey, sid)
		log.Printf("auth guard: authenticated user=%s session=%s device=%s", sub, sid, deviceID)
		c.Next()
	}
}
