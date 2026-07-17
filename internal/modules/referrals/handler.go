package referrals

import (
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
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

func (h *Handler) RedeemReferralCode(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var req RedeemReferralCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if err := h.service.RedeemReferralCode(c.Request.Context(), mobileUserID, strings.TrimSpace(req.ReferralCode)); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(200, response.APIResponse[any]{
		Status:  "success",
		Message: "Referral code has been successfully redeemed",
	})
}
