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
		return
	}

	transactions, err := h.service.FetchRecentTransactions(c.Request.Context(), mobileUserID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       true,
		"transactions": transactions,
	})
}

func (h *Handler) FetchAllTransactions(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var query FetchAllTransactionsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}

	result, err := h.service.FetchTransactionsPaged(c.Request.Context(), mobileUserID, query.Cursor, query.Limit)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
