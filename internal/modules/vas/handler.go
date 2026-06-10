package vas

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
	vasprovider "neat_mobile_app_backend/providers/vas"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetAirtime(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req AirtimePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.GetAirtime(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.ISPResponse]{
		Status:  "success",
		Message: "Airtime purchased successfully",
		Data:    &result,
	})
}

func (h *Handler) GetData(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req DataPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.GetData(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.ISPResponse]{
		Status:  "success",
		Message: "Data purchased successfully",
		Data:    &result,
	})
}

func (h *Handler) ValidateElectricity(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req ElectricityValidationPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.ValidateElectricity(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.ElectricityValidationResponse]{
		Status:  "success",
		Message: "Electricity account validated successfully",
		Data:    &result,
	})
}

func (h *Handler) PayElectricity(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req PayElectricityPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("vas handler: failed to bind pay electricity request - %s\n", err)
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.PayElectricity(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.PayElectricityResponse]{
		Status:  "success",
		Message: "Electricity bill paid successfully",
		Data:    &result,
	})
}

func (h *Handler) ValidateCable(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req ValidateCablePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.ValidateCable(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.CableValidationResponse]{
		Status:  "success",
		Message: "Cable account validated successfully",
		Data:    &result,
	})
}

func (h *Handler) PayCable(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req PayCablePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, err := h.service.PayCable(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*vasprovider.PayCableResponse]{
		Status:  "success",
		Message: "Cable bill paid successfully",
		Data:    &result,
	})
}
