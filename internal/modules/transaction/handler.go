package transaction

import (
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
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

func (h *Handler) FetchTransactionByID(c *gin.Context) {
	txID := strings.TrimSpace(c.Param("txID"))
	if txID == "" {
		mapped := response.MapError(appErr.ErrInvalidQueryParameter)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	tx, err := h.service.FetchTransactionByID(c.Request.Context(), txID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*TransactionResponse]{
		Status:  "success",
		Message: "Transaction fetched successfully",
		Data:    &tx,
	})
}

func (h *Handler) FetchRecentTransactions(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	transactions, err := h.service.FetchRecentTransactions(c.Request.Context(), mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]TransactionResponse]{
		Status:  "success",
		Message: "Recent transactions fetched successfully",
		Data:    &transactions,
	})
}

func (h *Handler) FetchAllTransactions(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var query FetchAllTransactionsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		mapped := response.MapError(appErr.ErrInvalidQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	result, err := h.service.FetchTransactionsPaged(c.Request.Context(), mobileUserID, query.Cursor, query.Limit)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[PagedTransactionResponse]{
		Status:  "success",
		Message: "Transactions fetched successfully",
		Data:    result,
	})
}
