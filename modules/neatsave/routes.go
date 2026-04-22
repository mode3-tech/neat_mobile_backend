package neatsave

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, authGuard gin.HandlerFunc, handler *Handler) {
	savings := rg.Group("/savings", authGuard)

	{
		savings.POST("/create/goal", handler.CreateGoal)
	}
}
