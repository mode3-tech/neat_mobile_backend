package reporting

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const requestTimeout = 30 * time.Second

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func withTimeout(c *gin.Context) (context.Context, context.CancelFunc) {
	if c == nil || c.Request == nil {
		return context.WithTimeout(context.Background(), requestTimeout)
	}
	return context.WithTimeout(c.Request.Context(), requestTimeout)
}

func (h *Handler) ListSignedUsers(c *gin.Context) {
	var query ListSignedUsersQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}

	ctx, cancel := withTimeout(c)
	defer cancel()

	resp, err := h.service.ListSignedUsers(ctx, query.Page, query.Limit)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "request timed out"})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	default:
		c.JSON(http.StatusOK, resp)
	}
}

func (h *Handler) GetUserTransactions(c *gin.Context) {
	var query UserTransactionQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}

	ctx, cancel := withTimeout(c)
	defer cancel()

	resp, err := h.service.GetUserTransactions(ctx, query.MobileUserID, query.Limit, query.Page)
	if err != nil {
		if isDBError(err) {
			if err.Error() == "missing user id" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "db lookup error"})
			return
		}
		if isUnprocessableEntityError(err) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

func isUnprocessableEntityError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "missing user id":
		return true
	default:
		return false
	}
}

func isDBError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "SQL") {
		return true
	}
	switch msg {
	case "an error occured when trying to fetch user transactions":
		return true
	default:
		return false
	}
}
