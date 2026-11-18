package wallet

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc) {
	wallet := rg.Group("/wallet", authGuard, deviceValidator)
	{
		wallet.GET("/banks", handler.FetchBanks)
		wallet.GET("/bank/details", handler.FetchBankDetails)
		wallet.POST("/transfer", handler.InitiateTransfer)
		wallet.POST("/beneficiary", handler.AddBeneficiary)
		wallet.GET("/beneficiaries", handler.GetBeneficiaries)

		// Wema ALAT-only: the provider's own history and statements.
		wallet.GET("/wema/history", handler.GetWemaTransactionHistory)
		wallet.POST("/wema/statement", handler.InitiateWemaStatement)
		wallet.GET("/wema/statement/:statement_id", handler.GetWemaStatement)
	}

}

// RegisterV2AccountStatementRoutes keeps the public route provider-neutral;
// v2 account statements are served from ALAT and require a Wema wallet.
func RegisterV2AccountStatementRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc) {
	account := rg.Group("/account", authGuard, deviceValidator)
	{
		account.POST("/statement/", handler.InitiateWemaStatement)
		account.GET("/statement/:statement_id", handler.GetWemaStatement)
	}
}

func RegisterV2BankRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc) {
	wallet := rg.Group("/wallet", authGuard, deviceValidator)
	{
		wallet.GET("/banks", handler.FetchWemaBanks)
		wallet.GET("/bank/details", handler.FetchWemaBankDetails)
	}
}

func RegisterWebhookRoutes(rg *gin.RouterGroup, handler *Handler, webhookAuth gin.HandlerFunc) {
	providus := rg.Group("/providus", webhookAuth)
	{
		providus.POST("/notification", handler.HandleBaaSEvent)
	}
}

// RegisterAlatCallbackRoutes mounts the original Authentication Callback path
// (/wallet/wema/authenticate). Production uses /alat/auth (see
// RegisterAlatWebhookRoutes); this alias stays until no ALAT environment is
// profiled with the old URL.
func RegisterAlatCallbackRoutes(rg *gin.RouterGroup, handler *Handler, webhookAuth gin.HandlerFunc) {
	rg.POST("/wema/authenticate", webhookAuth, handler.HandleAlatAuthentication)
}

// RegisterAlatWebhookRoutes mounts ALAT's callbacks under /alat - the URLs
// profiled with the bank. Wema signs none of its calls; webhookAuth restricts
// them to Wema's IPs. Handlers still don't act on a payload alone: the
// Authentication Callback checks the securityInfo HMAC we issued, the others
// confirm with ALAT first.
func RegisterAlatWebhookRoutes(rg *gin.RouterGroup, handler *Handler, webhookAuth gin.HandlerFunc) {
	alat := rg.Group("/alat", webhookAuth)
	{
		alat.POST("/account/creation", handler.HandleAlatAccountCreation)
		alat.POST("/auth", handler.HandleAlatAuthentication)
		alat.POST("/transaction", handler.HandleAlatTransaction)
		alat.POST("/transaction/notification", handler.HandleAlatTransactionNotification)
	}
}

// RegisterInternalRoutes mounts back-office wallet routes behind internalAuth.
func RegisterInternalRoutes(rg *gin.RouterGroup, handler *Handler, internalAuth gin.HandlerFunc) {
	internal := rg.Group("/wallet", internalAuth)
	{
		internal.POST("/wema/fund", handler.FundWemaWallet)
	}
}
