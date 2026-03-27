package loanproduct

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type InternalHandler struct {
	service *InternalService
}

func NewInternalHandler(service *InternalService) *InternalHandler {
	return &InternalHandler{service: service}
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

	resp, err := h.service.GetLoanApplicationsForCBA(c.Request.Context(), query.UserID)
	if err != nil {
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

	resp, err := h.service.GetLoanApplicationForCBA(c.Request.Context(), strings.TrimSpace(c.Param("application_ref")))
	switch {
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

	resp, err := h.service.GetEmbryoLoanApplicationsForCBA(c.Request.Context(), query.Page, query.Limit)
	if err != nil {
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

	resp, err := h.service.GetLoanApplicationBVNRecordForCBA(c.Request.Context(), query.UserID)

	if err != nil {
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

	var req UpdateCustomerStatusRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal loan callback service not configured"})
		return
	}

	err = h.service.ApplyCBACustomerStatusUpdate(c.Request.Context(), customerID, req, payload)
	switch {
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

	err = h.service.ApplyCBAStatusUpdate(c.Request.Context(), applicationRef, req, payload)
	switch {
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

	resp, err := h.service.LinkWalletUserByBVN(c.Request.Context(), req)
	switch {
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
