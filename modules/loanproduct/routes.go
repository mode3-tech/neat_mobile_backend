package loanproduct

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	loanProduct := rg.Group("/loan", authGuard)

	{
		loanProduct.GET("", handler.GetLoanProducts)
		loanProduct.POST("/apply", handler.ApplyForLoan)
		loanProduct.GET("/loans", handler.GetAllLoans)
		loanProduct.GET("/loans/active", handler.GetActiveLoans)
		loanProduct.GET("/loans/:loan_id", handler.GetLoanDetails)
		loanProduct.GET("/history", handler.GetLoanHistory)
		loanProduct.GET("/history/:loan_id", handler.GetLoanHistoryByLoanID)
		loanProduct.GET("/repayment-schedule", handler.GetRepaymentSchedule)
		loanProduct.POST("/repayment/manual", handler.HandleManualRepayment)
	}
}

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *InternalHandler, internalAuth gin.HandlerFunc) {
	cba := rg.Group("/cba")
	cba.Use(internalAuth)

	{
		cba.GET("/loan-applications", handler.GetLoanApplicationsForCBA)
		cba.GET("/loan-applications/embryo", handler.GetEmbryoLoanApplicationsForCBA)
		cba.GET("/loan-applications/:application_ref", handler.GetLoanApplicationForCBA)
		cba.PATCH("/loan-applications/:application_ref/status", handler.UpdateApplicationStatusFromCBA)
		cba.GET("/customers/bvn-record", handler.GetLoanApplicationBVNRecordForCBA)
		cba.POST("/customers/link-by-bvn", handler.LinkWalletUserByBVN)
		cba.PATCH("/customers/:customer_id/status", handler.UpdateCustomerStatusFromCBA)
	}
}
