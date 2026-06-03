package transaction

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc) {
	tx := rg.Group("/transaction", authGuard, deviceValidator)
	{
		tx.GET("/recent", handler.FetchRecentTransactions)
		tx.GET("/all", handler.FetchAllTransactions)
	}
}
