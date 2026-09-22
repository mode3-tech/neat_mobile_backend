package tier

import (
	"log"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
	"net/http"
	"strings"

	appErr "neat_mobile_app_backend/internal/errors"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) ValidateNIN(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		log.Println("tier handler: missing user id in context for ValidateNIN")
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	err := h.service.ValidateNIN(c.Request.Context(), mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "NIN successfully verified",
	})

}

func (h *Handler) ValidatePassportPhotograph(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		log.Println("tier handler: missing user id in context for FetchBillersByCategoryID")
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	var req ValidatePassportPhotographRequest
	if err := c.BindJSON(&req); err != nil {
		log.Println("tier handler: invalid request body for ValidatePassportPhotograph")
		mapped := response.MapError(appErr.ErrBadRequest)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{Status: "error", Error: &mapped.Error})
		return
	}

	err := h.service.ValidateNINWithFace(c.Request.Context(), mobileUserID, req.Image)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Passport photograph validated successfully",
	})
}
