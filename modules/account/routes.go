package account

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	account := rg.Group("/account", authGuard)
	{
		account.GET("/summary", handler.GetAccountSummary)
		account.PATCH("/profile", handler.UpdateProfile)
		account.POST("/statement", handler.GetAccountStatement)
		account.GET("/statement/:job_id/status", handler.GetStatementJobStatus)
		account.GET("/statement/latest", handler.GetLatestAccountStatement)
	}
}
