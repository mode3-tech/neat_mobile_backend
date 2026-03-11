package loanproduct

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	loanProduct := rg.Group("/loan-product")

	{
		loanProduct.GET("/", authGuard, handler.GetLoanProducts)
	}
}
