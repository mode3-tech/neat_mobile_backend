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
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
	}

	resp, err := h.service.CreateGoal(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		handleNeatSaveError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetUserGoals(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.service.GetUserGoals(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		handleNeatSaveError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetGoalSummary(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var query GetGoalSummaryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}

	resp, err := h.service.GetGoalSummary(c.Request.Context(), mobileUserID, deviceID, strings.TrimSpace(query.GoalID))
	if err != nil {
		handleNeatSaveError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleNeatSaveError(c *gin.Context, err error) {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "mobile user id is required", "device not found", "device not allowed":
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	case "goal's name can not be empty":
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case "target amount and auto save amount can not be less that NGN 50", "target date must be in the future":
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case "error creating savings goal", "error fetching user goals":
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		return
	}
}
