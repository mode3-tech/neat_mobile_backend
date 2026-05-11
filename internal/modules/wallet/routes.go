package wallet

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	wallet := rg.Group("/wallet", authGuard)
	{
		wallet.GET("/banks", handler.FetchBanks)
		wallet.GET("/bank/details", handler.FetchBankDetails)
		wallet.POST("/transfer", handler.InitiateTransfer)
		wallet.POST("/beneficiary", handler.AddBeneficiary)
		wallet.GET("/beneficiaries", handler.GetBeneficiaries)
		wallet.POST("/transfer/bulk", handler.InitiateBulkTransfer)
	}

}

func RegisterWebhookRoutes(rg *gin.RouterGroup, handler *Handler, webhookAuth gin.HandlerFunc) {
	providus := rg.Group("/providus", webhookAuth)
	{
		providus.POST("/credit", handler.HandleCreditWebhook)
	}
}
