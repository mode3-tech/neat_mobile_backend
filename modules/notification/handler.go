package notification

import (
	"errors"
	"neat_mobile_app_backend/internal/middleware"
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
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.RegisterToken(c.Request.Context(), userID, req); err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to register push token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "push token registered"})
}

func (h *Handler) DeleteToken(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	if err := h.service.DeleteToken(c.Request.Context(), userID, deviceID); err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to delete push token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "push token deleted"})
}

func (h *Handler) SendNotification(c *gin.Context) {
	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

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
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	var query ListNotificationsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}
	response, err := h.service.GetNotifications(c.Request.Context(), userID, query.Page, query.PageSize)
	if err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "unable to retrieve notifications"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	count, err := h.service.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "unable to retrieve unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	notificationID := strings.TrimSpace(c.Param("id"))
	updated, err := h.service.MarkNotificationRead(c.Request.Context(), userID, notificationID)
	if err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "unable to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "notification updated",
		"updated": updated,
	})
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.service == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "notification service not configured"})
		return
	}

	updated, err := h.service.MarkAllNotificationsRead(c.Request.Context(), userID)
	if err != nil {
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "unable to mark notifications as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "notifications updated",
		"updated": updated,
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
