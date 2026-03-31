package wallet

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	wallet := rg.Group("/wallet")
	{
		wallet.GET("/banks", handler.FetchBanks)
		wallet.GET("/bank/details", handler.FetchBankDetails)
	}
}
