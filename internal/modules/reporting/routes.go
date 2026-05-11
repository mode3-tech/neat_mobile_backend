package reporting

import "github.com/gin-gonic/gin"

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *Handler, internalAuth gin.HandlerFunc) {
	reporting := rg.Group("/reporting")
	reporting.Use(internalAuth)

	{
		reporting.GET("/users", handler.ListSignedUsers)
		reporting.GET("/user/transaction", handler.GetUserTransactions)
	}
}
