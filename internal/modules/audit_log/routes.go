package auditlog

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/audit/logs", handler.GetAuditLogs)
}
