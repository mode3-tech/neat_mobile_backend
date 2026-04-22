package neatsave

import (
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

func (h *Handler) CreateGoal(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req CreateGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
	}

	resp, err := h.service.CreateGoal(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		if isUnauthorizedError(err) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if isBadRequestError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isUnprocessableEntityError(err) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		if isInternalServerError(err) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "mobile user id is required", "device not found", "device not allowed":
		return true
	default:
		return false
	}
}

func isBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "goal's name can not be empty":
		return true
	default:
		return false
	}
}

func isUnprocessableEntityError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "target amount and auto save amount can not be less that NGN 50", "target date must be in the future":
		return true
	default:
		return false
	}
}

func isInternalServerError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "error creating savings goal":
		return true
	default:
		return false
	}
}
