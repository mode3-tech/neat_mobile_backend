package card

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, authGuard gin.HandlerFunc, handler *Handler) {
	card := rg.Group("/card")

	{
		card.POST("/request", authGuard, handler.RequestForCard)
	}
}
