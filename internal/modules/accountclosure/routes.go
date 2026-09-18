package accountclosure

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, authGuard, deviceValidator gin.HandlerFunc, handler *Handler) {
	accountclosure := rg.Group("/account/close", authGuard, deviceValidator)

	accountclosure.POST("", handler.CloseAccount)
}
