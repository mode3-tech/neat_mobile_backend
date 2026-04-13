package account

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

func (h *Handler) GetAccountSummary(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	summary, err := h.service.GetAccountSummary(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch account summary"})
		return
	}

	c.JSON(http.StatusOK, AccountSummaryResponse{
		Status: true,
		Data:   *summary,
	})
}

func (h *Handler) GetAccountStatement(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	var req AccountStatementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	jobID, err := h.service.RequestAccountStatement(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to request account statement"})
		return
	}

	c.JSON(http.StatusOK, AccountStatementResponse{
		Status:  true,
		Message: "Account statement request is being processed",
		JobID:   jobID,
	})
}

func (h *Handler) GetStatementJobStatus(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	job, downloadURL, err := h.service.GetStatementJobStatus(c.Request.Context(), mobileUserID, jobID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, StatementJobStatusResponse{
		Status:      true,
		JobStatus:   string(job.Status),
		DownloadURL: downloadURL,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	if err := h.service.UpdateProfile(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		if err.Error() == "user id is missing" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	c.JSON(http.StatusOK, &UpdateProfileResponse{
		Status:  true,
		Message: "Profile successfully updated",
	})
}
