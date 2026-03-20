package loanproduct

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	loanProduct := rg.Group("/loan")

	{
		loanProduct.GET("", handler.GetLoanProducts)
		loanProduct.POST("/apply", authGuard, handler.ApplyForLoan)
		loanProduct.GET("/loans", authGuard, handler.GetAllLoans)
		loanProduct.GET("/repayment-schedule", authGuard, handler.GetRepaymentSchedule)
	}
}

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *InternalHandler, internalAuth gin.HandlerFunc) {
	cba := rg.Group("/cba")
	cba.Use(internalAuth)

	{
		cba.GET("/loan-applications", handler.GetLoanApplicationsForCBA)
		cba.PATCH("/loan-applications/:application_ref/status", handler.UpdateApplicationStatusFromCBA)
	}
}
