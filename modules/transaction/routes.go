package transaction

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	tx := rg.Group("/transaction", authGuard)
	{
		tx.GET("/recent", handler.FetchRecentTransactions)
	}
}
