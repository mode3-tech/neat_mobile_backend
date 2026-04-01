package wallet

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	wallet := rg.Group("/wallet", authGuard)
	{
		wallet.GET("/banks", handler.FetchBanks)
		wallet.GET("/bank/details", handler.FetchBankDetails)
		wallet.POST("/transfer", handler.InitiateTransfer)
	}
}
