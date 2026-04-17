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

	deviceID := c.GetHeader("X-Device-ID")

	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	job, downloadURL, err := h.service.GetStatementJobStatus(c.Request.Context(), mobileUserID, deviceID, jobID)
	if err != nil {
		if isUnauthorizedGetStatementJobStatusError(err) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if isInternalServerGetStatementJobStatusError(err) {
			if err.Error() == "job not found" {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve account report"})
			return
		}
		if isBadRequestGetStatementJobStatusError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, StatementJobStatusResponse{
		Status:      true,
		JobStatus:   string(job.Status),
		DownloadURL: downloadURL,
	})
}

func isUnauthorizedGetStatementJobStatusError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "mobile user id is required", "device id is required":
		return true
	}
	return false
}

func isInternalServerGetStatementJobStatusError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "job not found":
		return true
	}
	if strings.HasPrefix(msg, "failed to retrieve account report job: ") {
		return true
	}
	return false
}

func isBadRequestGetStatementJobStatusError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "job id is required":
		return true
	}
	return false
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

func (h *Handler) GetLatestAccountStatement(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	resp, err := h.service.GetLatestAccountStatement(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isUnauthorizedGetLatestAccountStatementError(err) {
			if err.Error() == "device not found" {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if isInternalServerGetLatestAccountStatementError(err) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": strings.SplitN(err.Error(), ":", 2)[0]})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isUnauthorizedGetLatestAccountStatementError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "mobile user id or device id is missing", "device not found":
		return true
	}
	return false
}

func isInternalServerGetLatestAccountStatementError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	if strings.HasPrefix(msg, "failed to generate download link for account statement: ") || strings.HasPrefix(msg, "failed to save download URL for account statement: ") {
		return true
	}
	return false
}
