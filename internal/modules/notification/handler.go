package notification

import (
	"errors"
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

func (h *Handler) RegisterToken(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if err := h.service.RegisterToken(c.Request.Context(), mobileUserID, req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Push token registered",
	})
}

func (h *Handler) DeleteToken(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if err := h.service.DeleteToken(c.Request.Context(), mobileUserID, deviceID); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Push token deleted",
	})
}

func (h *Handler) SendNotification(c *gin.Context) {
	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.SendToUserWithOptions(c.Request.Context(), req); err != nil {
		switch {
		case isBadRequestError(err):
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSenderNotConfigured):
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "failed to send notification"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification sent"})
}

func (h *Handler) GetNotifications(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	var query ListNotificationsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
	resp, err := h.service.GetNotifications(c.Request.Context(), mobileUserID, deviceID, query.Page, query.PageSize)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]NotificationDTO]{
		Status:  "success",
		Message: "Notifications fetched successfully",
		Data:    &resp.Notifications,
		Page:    &resp.Page,
		Limit:   &resp.Limit,
		Total:   &resp.Total,
	})
}

func (h *Handler) GetUnreadCount(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	count, err := h.service.GetUnreadCount(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := &UnreadNotificationCountResponse{
		Count: count,
	}

	c.JSON(http.StatusOK, response.APIResponse[UnreadNotificationCountResponse]{
		Status:  "success",
		Message: "Unread notification count was successful",
		Data:    dto,
	})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var params NotificationParams

	if err := c.ShouldBindUri(&params); err != nil {
		mapped := response.MapError(appErr.ErrMissingRequiredPathParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	updated, err := h.service.MarkNotificationRead(c.Request.Context(), mobileUserID, deviceID, strings.TrimSpace(params.ID))
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := MarkNotificationReadResponse{
		Updated: &updated,
	}

	c.JSON(http.StatusOK, response.APIResponse[MarkNotificationReadResponse]{
		Status:  "success",
		Message: "Notification(s) marked as read",
		Data:    &dto,
	})
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	err := h.service.MarkAllNotificationsRead(c.Request.Context(), mobileUserID, deviceID)
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
		Message: "Notifications successfully marked as read",
	})
}

func (h *Handler) TogglePushNotification(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.TogglePushNotifications(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := &TogglePushNotificationsResponse{
		IsEnabled: resp.IsEnabled,
	}

	c.JSON(http.StatusOK, response.APIResponse[TogglePushNotificationsResponse]{
		Status:  "success",
		Message: resp.Message,
		Data:    dto,
	})
}

func isBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "user id is required",
		"device id is required",
		"expo push token is required",
		"platform must be ios or android",
		"invalid expo push token",
		"title is required",
		"body is required",
		"notification id is required",
		"notification type is invalid":
		return true
	default:
		return false
	}
}

func isUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed":
		return true
	default:
		return false
	}
}
