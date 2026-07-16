package referrals

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, authGuard, deviceValidator gin.HandlerFunc, handler *Handler) {
	referral := rg.Group("/referrals")
	referral.Use(authGuard, deviceValidator)

	{
		referral.POST("/redeem", handler.RedeemReferralCode)
	}

}
