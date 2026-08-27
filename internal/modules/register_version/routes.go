package registerversion

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, handler *Handler) {
	r.GET("/register-version", handler.GetRegisterVersion)
}
