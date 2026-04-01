package account

import (
	"neat_mobile_app_backend/internal/middleware"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetAccountSummary(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	summary, err := h.service.GetAccountSummary(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch account summary"})
		return
	}

	c.JSON(http.StatusOK, AccountSummaryResponse{
		Status: true,
		Data:   *summary,
	})
}
