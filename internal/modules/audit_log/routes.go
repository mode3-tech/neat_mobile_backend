package auditlog

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	r.GET("/audit/logs", h.GetAuditLogs)
}
