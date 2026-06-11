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

func (h *Handler) FetchAllCategories(c *gin.Context) {
	categories, err := h.service.FetchAllCategories(c.Request.Context())
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]vasprovider.Category]{
		Status:  "success",
		Message: "VAS categories fetched successfully.",
		Data:    &categories,
	})
}

func (h *Handler) FetchBillersByCategoryID(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		log.Println("vas handler: missing user id in context for FetchBillersByCategoryID")
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req FetchBillersByCategoryIDQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, page, size, totalCount, totalPages, hasNext, hasPrev, err := h.service.FetchBillingsByCategoryID(c.Request.Context(), BillingsByCategoryIDPayload{
		CategoryID: req.CategoryID,
	}, req.Size, req.Page)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[BillingsByCategoryIDResponse]{
		Status:     "success",
		Message:    "VAS billers fetched successfully.",
		Data:       result,
		Page:       &page,
		Size:       &size,
		TotalCount: &totalCount,
		TotalPages: &totalPages,
		HasNext:    &hasNext,
		HasPrev:    &hasPrev,
	})
}

func (h *Handler) FetchProducts(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		log.Println("vas handler: missing user id in context for FetchProducts")
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req FetchProductsQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Printf("vas handler: %s\n", err)
		mapped := response.MapError(appErr.ErrInvalidQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	result, page, size, totalCount, totalPages, hasNext, hasPrev, err := h.service.FetchProductsByCategoryIDAndBillerID(c.Request.Context(), FetchProductsByCategoryIDAndBillerIDPayload{
		CategoryID: req.CategoryID,
		BillerID:   req.BillerID,
	}, req.Size, req.Page)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ProductsResponse]{
		Status:     "success",
		Message:    "VAS products fetched successfully.",
		Data:       result,
		Page:       &page,
		Size:       &size,
		TotalCount: &totalCount,
		TotalPages: &totalPages,
		HasNext:    &hasNext,
		HasPrev:    &hasPrev,
	})
}

func (h *Handler) GetAirtime(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		log.Println("vas handler: missing user id in context for GetAirtime")
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
		log.Println("vas handler: missing user id in context for GetData")
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
		log.Println("vas handler: missing user id in context for ValidateElectricity")
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
		log.Println("vas handler: missing user id in context for PayElectricity")
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
		log.Println("vas handler: missing user id in context for ValidateCable")
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
		log.Println("vas handler: missing user id in context for PayCable")
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
