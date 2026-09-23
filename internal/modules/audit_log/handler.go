package auditlog

import (
	"math"
	"neat_mobile_app_backend/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}
func (h *Handler) GetAuditLogs(c *gin.Context) {
	var auditLogsQuery GetAuditLogsQuery
	if err := c.ShouldBindQuery(&auditLogsQuery); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	filters := make(map[string]interface{})

	switch {
	case auditLogsQuery.ActorType != "":
		filters["actor_type"] = auditLogsQuery.ActorType
	case auditLogsQuery.ActorID != "":
		filters["actor_id"] = auditLogsQuery.ActorID
	case auditLogsQuery.Action != "":
		filters["action"] = auditLogsQuery.Action
	case auditLogsQuery.ResourceType != "":
		filters["resource_type"] = auditLogsQuery.ResourceType
	case auditLogsQuery.ResourceID != "":
		filters["resource_id"] = auditLogsQuery.ResourceID
	case auditLogsQuery.Status != "":
		filters["status"] = auditLogsQuery.Status
	case auditLogsQuery.RequestID != "":
		filters["request_id"] = auditLogsQuery.RequestID
	case auditLogsQuery.IPAddress != "":
		filters["ip_address"] = auditLogsQuery.IPAddress
	}

	auditLogs, count, err := h.Service.GetAuditLogs(c.Request.Context(), filters, auditLogsQuery.Page, auditLogsQuery.Limit)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	totalCount := len(auditLogs)
	totalPages := int(math.Ceil(float64(totalCount) / float64(auditLogsQuery.Limit)))

	c.JSON(http.StatusOK, response.APIResponse[[]AuditLog]{
		Status:     "success",
		Data:       &auditLogs,
		TotalCount: &totalCount,
		Total:      &count,
		TotalPages: &totalPages,
		Page:       &auditLogsQuery.Page,
		Limit:      &auditLogsQuery.Limit,
	})
}
