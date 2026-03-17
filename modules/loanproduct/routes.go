package loanproduct

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	loanProduct := rg.Group("/loan")

	{
		loanProduct.GET("/", authGuard, handler.GetLoanProducts)
		loanProduct.POST("/apply", authGuard, handler.ApplyForLoan)
	}
}

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *InternalHandler, internalAuth gin.HandlerFunc) {
	cba := rg.Group("/cba")
	cba.Use(internalAuth)

	{
		cba.PATCH("/loan-applications/:application_ref/status", handler.UpdateApplicationStatusFromCBA)
	}
}
