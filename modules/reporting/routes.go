package reporting

import "github.com/gin-gonic/gin"

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *Handler, internalAuth gin.HandlerFunc) {
	g := rg.Group("/reporting")
	g.Use(internalAuth)

	{
		g.GET("/users", handler.ListSignedUsers)
	}
}
