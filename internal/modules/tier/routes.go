package tier

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, authGuard, deviceGuard gin.HandlerFunc, handler *Handler) {
	tier := r.Group("/tier")

	{
		tier.POST("/validate/nin", authGuard, deviceGuard, handler.ValidateNIN)
		tier.POST("/validate/nin/face", authGuard, deviceGuard, handler.ValidatePassportPhotograph)
	}
}
