package account

import (
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

func (h *Handler) GetAccountSummary(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	summary, err := h.service.GetAccountSummary(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[AccountSummary]{
		Status:  "success",
		Message: "Account summary has been fetched successfully",
		Data:    summary,
	})
}

func (h *Handler) GetAccountStatement(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	var req AccountStatementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body",
			},
		})
		return
	}

	jobID, err := h.service.RequestAccountStatement(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := AccountStatementResponse{
		JobID: jobID,
	}

	c.JSON(http.StatusOK, response.APIResponse[AccountStatementResponse]{
		Status:  "success",
		Message: "Account statement generation has been initiated successfully",
		Data:    &dto,
	})
}

func (h *Handler) GetStatementJobStatus(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidParams),
				Message: "job_id is required",
			},
		})
		return
	}

	job, downloadURL, err := h.service.GetStatementJobStatus(c.Request.Context(), mobileUserID, deviceID, jobID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := StatementJobStatusResponse{
		JobStatus:   string(job.Status),
		DownloadURL: downloadURL,
	}

	c.JSON(http.StatusOK, response.APIResponse[StatementJobStatusResponse]{
		Status:  "success",
		Message: "Account statement job status has been fetched successfully",
		Data:    &dto,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBind(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body",
			},
		})
		return
	}

	file, header, err := c.Request.FormFile("profile_picture")
	if err != nil && err != http.ErrMissingFile {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid image file",
			},
		})
		return
	}

	var profilePictureURL *string

	if file != nil {
		defer file.Close()

		contentType := header.Header.Get("Content-Type")
		if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
			c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    string(ErrCodeInvalidRequestBody),
					Message: "only jpeg, png and webp allowed",
				},
			})
			return
		}

		if header.Size > 5<<20 {
			c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    string(ErrCodeInvalidRequestBody),
					Message: "image larger than 8MB",
				},
			})
			return
		}
		uploadedURL, uploadErr := h.service.uploadProfilePicture(c.Request.Context(), file, *header, mobileUserID)
		if uploadErr != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    string(ErrCodeInternalServerError),
					Message: "an error occured while uploading profile picture",
				},
			})
			return
		}
		profilePictureURL = &uploadedURL
	} else if req.RemoveProfilePicture {
		empty := ""
		profilePictureURL = &empty
	}

	if err := h.service.UpdateProfile(c.Request.Context(), mobileUserID, deviceID, profilePictureURL, req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "Profile has been updated successfully",
	})
}

func (h *Handler) GetLatestAccountStatement(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	resp, err := h.service.GetLatestAccountStatement(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[GetLatestAccountStatementResponse]{
		Status:  "success",
		Message: "Latest account statement has been fetched successfully",
		Data:    resp,
	})
}
