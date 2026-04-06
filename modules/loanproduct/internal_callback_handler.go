package loanproduct

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const internalCallbackRequestTimeout = 30 * time.Second

type InternalHandler struct {
	service *InternalService
}

func NewInternalHandler(service *InternalService) *InternalHandler {
	return &InternalHandler{service: service}
}

func withInternalCallbackContext(c *gin.Context) (context.Context, context.CancelFunc) {
	if c == nil || c.Request == nil {
		return context.WithTimeout(context.Background(), internalCallbackRequestTimeout)
	}

	return context.WithTimeout(c.Request.Context(), internalCallbackRequestTimeout)
}

func handleInternalCallbackTimeout(c *gin.Context, err error) bool {
	if !errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "internal callback request timed out"})
	return true
}

func (h *InternalHandler) GetLoanApplicationsForCBA(c *gin.Context) {
	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	var query LoanApplicationsForCBAQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	resp, err := h.service.GetLoanApplicationsForCBA(ctx, query.UserID)
	if err != nil {
		if handleInternalCallbackTimeout(c, err) {
			return
		}
		if errors.Is(err, ErrInvalidMobileUserID) || errors.Is(err, ErrBadRequest) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InternalHandler) GetLoanApplicationForCBA(c *gin.Context) {
	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	resp, err := h.service.GetLoanApplicationForCBA(ctx, strings.TrimSpace(c.Param("application_ref")))
	switch {
	case handleInternalCallbackTimeout(c, err):
		return
	case errors.Is(err, ErrApplicationNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrBadRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	default:
		c.JSON(http.StatusOK, resp)
	}
}

func (h *InternalHandler) GetEmbryoLoanApplicationsForCBA(c *gin.Context) {
	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	var query EmbryoLoanApplicationsForCBAQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	resp, err := h.service.GetEmbryoLoanApplicationsForCBA(ctx, query.Page, query.Limit)
	if err != nil {
		if handleInternalCallbackTimeout(c, err) {
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InternalHandler) GetLoanApplicationBVNRecordForCBA(c *gin.Context) {
	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	var query LoanApplicationBVNRecordQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	resp, err := h.service.GetLoanApplicationBVNRecordForCBA(ctx, query.UserID)

	if err != nil {
		if handleInternalCallbackTimeout(c, err) {
			return
		}
		if isUnprocessableEntityBVNRecordError(err) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		if isRecordNotFoundBVNRecordError(err) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isUnprocessableEntityBVNRecordError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "bad request", "invalid mobile user id":
		return true
	default:
		return false
	}
}

func isRecordNotFoundBVNRecordError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "customer record not found":
		return true
	default:
		return false
	}
}

func (h *InternalHandler) UpdateCustomerStatusFromCBA(c *gin.Context) {
	customerID := strings.TrimSpace(c.Param("customer_id"))

	payload, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var req UpdateCustomerRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	err = h.service.ApplyCBACustomerUpdate(ctx, customerID, req, payload)
	switch {
	case handleInternalCallbackTimeout(c, err):
		return
	case errors.Is(err, ErrCustomerNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidCustomerID), errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrBadRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidCustomerTransition):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	default:
		c.JSON(http.StatusOK, gin.H{"message": "customer status updated"})
	}
}

func (h *InternalHandler) UpdateApplicationStatusFromCBA(c *gin.Context) {
	applicationRef := strings.TrimSpace(c.Param("application_ref"))

	payload, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var req UpdateLoanApplicationStatusRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	err = h.service.ApplyCBAStatusUpdate(ctx, applicationRef, req, payload)
	switch {
	case handleInternalCallbackTimeout(c, err):
		return
	case errors.Is(err, ErrApplicationNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrBadRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidTransition):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	default:
		c.JSON(http.StatusOK, gin.H{"message": "loan application updated"})
	}
}

func (h *InternalHandler) LinkWalletUserByBVN(c *gin.Context) {
	var req LinkWalletUserByBVNRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	ctx, cancel := withInternalCallbackContext(c)
	defer cancel()

	resp, err := h.service.LinkWalletUserByBVN(ctx, req)
	switch {
	case handleInternalCallbackTimeout(c, err):
		return
	case errors.Is(err, ErrBadRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	default:
		c.JSON(http.StatusOK, gin.H{
			"message": "wallet user linkage processed",
			"data":    resp,
		})
	}
}
