package transaction

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

func (h *Handler) FetchRecentTransactions(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)

	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
	}

}
