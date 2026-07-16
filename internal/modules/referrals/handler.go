package referrals

import (
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
	return &Handler{service: service}
}

func (h *Handler) GenerateReferralCode(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.JSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	referralCode, err := h.service.GenerateReferralCode(c.Request.Context(), mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.JSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := GenerateReferralCodeResponse{ReferralCode: referralCode}

	c.JSON(http.StatusOK, response.APIResponse[GenerateReferralCodeResponse]{
		Status:  "success",
		Message: "Referral code generated successfully",
		Data:    &dto,
	})
}
