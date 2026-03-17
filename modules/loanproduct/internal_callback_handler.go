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
