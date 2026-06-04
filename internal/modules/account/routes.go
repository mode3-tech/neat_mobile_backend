package account

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc) {
	account := rg.Group("/account", authGuard, deviceValidator)
	{
		account.GET("/summary", handler.GetAccountSummary)
		account.GET("/limits", handler.GetAccountLimits)
		account.PATCH("/profile", handler.UpdateProfile)
		account.POST("/statement", handler.GetAccountStatement)
		account.GET("/statement/:job_id/status", handler.GetStatementJobStatus)
		account.GET("/statement/latest", handler.GetLatestAccountStatement)
	}
}
