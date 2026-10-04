package auditlog

import (
	"math"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

func abortWithError(c *gin.Context, err error) {
	mapped := response.MapError(err)
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}

// parseDateParam accepts YYYY-MM-DD or RFC3339 and returns a UTC time.
// dateOnly reports whether the value had no time component. Empty input yields nil.
func parseDateParam(value string, invalidErr error) (t *time.Time, dateOnly bool, err error) {
	if value == "" {
		return nil, false, nil
	}
	if parsed, perr := time.ParseInLocation("2006-01-02", value, time.UTC); perr == nil {
		return &parsed, true, nil
	}
	if parsed, perr := time.Parse(time.RFC3339, value); perr == nil {
		parsed = parsed.UTC()
		return &parsed, false, nil
	}
	return nil, false, invalidErr
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

	if auditLogsQuery.ActorType != "" {
		filters["actor_type"] = auditLogsQuery.ActorType
	}
	if auditLogsQuery.ActorID != "" {
		filters["actor_id"] = auditLogsQuery.ActorID
	}
	if auditLogsQuery.Action != "" {
		filters["action"] = auditLogsQuery.Action
	}
	if auditLogsQuery.ResourceType != "" {
		filters["resource_type"] = auditLogsQuery.ResourceType
	}
	if auditLogsQuery.ResourceID != "" {
		filters["resource_id"] = auditLogsQuery.ResourceID
	}
	if auditLogsQuery.Status != "" {
		filters["status"] = auditLogsQuery.Status
	}
	if auditLogsQuery.RequestID != "" {
		filters["request_id"] = auditLogsQuery.RequestID
	}
	if auditLogsQuery.IPAddress != "" {
		filters["ip_address"] = auditLogsQuery.IPAddress
	}

	from, _, err := parseDateParam(auditLogsQuery.DateFrom, appErr.ErrInvalidDateFrom)
	if err != nil {
		abortWithError(c, err)
		return
	}
	to, toDateOnly, err := parseDateParam(auditLogsQuery.DateTo, appErr.ErrInvalidDateTo)
	if err != nil {
		abortWithError(c, err)
		return
	}
	if to != nil && toDateOnly {
		// A date-only upper bound is inclusive of that whole day.
		end := to.Add(24 * time.Hour)
		to = &end
	}
	if from != nil && to != nil && from.After(*to) {
		abortWithError(c, appErr.ErrInvalidDateRange)
		return
	}

	auditLogs, count, err := h.Service.GetAuditLogs(c.Request.Context(), filters, from, to, auditLogsQuery.Page, auditLogsQuery.Limit)
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
