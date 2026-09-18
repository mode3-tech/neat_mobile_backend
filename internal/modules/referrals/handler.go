package referrals

import (
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
	"strconv"
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

func (h *Handler) FetchRedeemReferrals(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "20")

	pageInt, err := strconv.Atoi(page)
	if err != nil {
		pageInt = 1
	}
	pageSizeInt, err := strconv.Atoi(pageSize)
	if err != nil {
		pageSizeInt = 20
	}

	redeemedReferrals, err := h.service.FetchRedeemReferrals(c.Request.Context(), pageInt, pageSizeInt)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := make([]RedeemedReferralResponse, len(redeemedReferrals))
	for i, v := range redeemedReferrals {
		dto[i] = RedeemedReferralResponse{
			ID:           v.ID,
			ReferrerName: v.ReferrerName,
			ReferredName: v.ReferredName,
			RedeemedAt:   v.RedeemedAt,
		}
	}

	c.JSON(200, response.APIResponse[[]RedeemedReferralResponse]{
		Status: "success",
		Data:   &dto,
	})
}
