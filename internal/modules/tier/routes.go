package tier

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, authGuard, deviceGuard gin.HandlerFunc, handler *Handler) {
	tier := r.Group("/tier")

	{
		tier.POST("/validate/nin", authGuard, deviceGuard, handler.ValidateNIN)
		tier.POST("/validate/nin/face", authGuard, deviceGuard, handler.ValidatePassportPhotograph)

		// Wema ALAT-only bank-side upgrades.
		tier.POST("/upgrade/tier3", authGuard, deviceGuard, handler.UpgradeWemaTier3)
		tier.GET("/kyc-status", authGuard, deviceGuard, handler.GetWemaKYCStatus)
	}
}
