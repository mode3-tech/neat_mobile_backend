package appversion

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, handler *Handler) {
	r.GET("/app/version/:os", handler.GetAppVersion)
}
