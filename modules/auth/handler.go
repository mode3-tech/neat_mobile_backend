package auth

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

func (h *Handler) Register(c *gin.Context) {
	var req RegisterationRequest

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

	ip := c.ClientIP()

	resp, err := h.service.Register(c.Request.Context(), req, ip)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	statusCode := http.StatusAccepted
	if resp != nil && resp.RegistrationStatus == string(RegistrationJobStatusCompleted) {
		statusCode = http.StatusOK
	}

	c.JSON(statusCode, response.APIResponse[RegistrationJobResponse]{
		Status:  "success",
		Message: "Registration processed successfully.",
		Data:    resp,
	})
}

func (h *Handler) GetRegistrationStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidParams),
				Message: "Missing query paramter.",
			},
		})
		return
	}

	resp, err := h.service.GetRegistrationStatus(c.Request.Context(), jobID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[RegistrationJobResponse]{
		Status:  "success",
		Message: "Registration status retrieved successfully.",
		Data:    resp,
	})
}

func (h *Handler) ClaimRegistrationSession(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidParams),
				Message: "Missing query paramter.",
			},
		})
		return
	}

	var req RegistrationSessionClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	ip := c.ClientIP()

	resp, err := h.service.ClaimRegistrationSession(c.Request.Context(), jobID, req.ClaimToken, deviceID, ip)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if resp == nil || resp.AccessToken == "" || resp.RefreshToken == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "Registration session claimed successfully.",
		Data:    resp,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	ip := c.ClientIP()

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized.",
			},
		})
		return
	}

	loginObj, err := h.service.Login(c.Request.Context(), deviceID, ip, strings.TrimSpace(req.Phone), strings.TrimSpace(req.Password))

	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if loginObj == nil || loginObj.Status == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	resp := LoginInitResponse{
		Challenge:    loginObj.Challenge,
		SessionToken: loginObj.SessionToken,
	}

	c.JSON(http.StatusOK, response.APIResponse[LoginInitResponse]{
		Status:  "success",
		Message: "Login credentials verified.",
		Data:    &resp,
	})
}

func (h *Handler) VerifyDevice(c *gin.Context) {
	var req VerifyDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	ip := c.ClientIP()

	resp, err := h.service.VerifyDeviceChallenge(c.Request.Context(), req.Challenge, req.Signature, req.DeviceID, ip)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if resp == nil || resp.AccessToken == "" || resp.RefreshToken == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "device successfully verified",
		Data:    resp,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	authHeader := strings.TrimSpace(c.Request.Header.Get("Authorization"))
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
			},
		})
		return
	}
	splittedAuthHeader := strings.Fields(authHeader)
	if len(splittedAuthHeader) != 2 || splittedAuthHeader[0] != "Bearer" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
			},
		})
		return
	}

	accessToken := strings.TrimSpace(splittedAuthHeader[1])
	if accessToken == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken, accessToken); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "Logged out.",
	})
}

func (h *Handler) RefreshAccessToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	tokenObj, err := h.service.RefreshAccessToken(c.Request.Context(), strings.TrimSpace(req.DeviceID), strings.TrimSpace(req.RefreshToken))
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if tokenObj == nil || tokenObj.AccessToken == "" || tokenObj.RefreshToken == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[LoginResponse]{
		Status:  "success",
		Message: "access token successfully refreshed",
		Data:    (*LoginResponse)(tokenObj),
	})
}

func (h *Handler) VerifyBVN(c *gin.Context) {
	var req BVNValidationRequest
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

	bvnInfo, err := h.service.ValidateBVN(c.Request.Context(), req.BVN)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if bvnInfo == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	resp := &BVNValidationResponse{
		Name:           bvnInfo.name,
		DOB:            bvnInfo.dob,
		PhoneNumber:    bvnInfo.phone,
		VerificationID: bvnInfo.verificationID,
	}

	c.JSON(http.StatusOK, response.APIResponse[BVNValidationResponse]{
		Status:  "success",
		Message: "BVN verification was succesful.",
		Data:    resp,
	})
}

func (h *Handler) VerifyNIN(c *gin.Context) {
	var req NINValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	ninInfo, err := h.service.ValidateNIN(c.Request.Context(), req.BVNVerificationID, req.NIN)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if ninInfo == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	resp := &NINValidationResponse{
		Name:           ninInfo.name,
		DOB:            ninInfo.dob,
		PhoneNumber:    ninInfo.phone,
		VerificationID: ninInfo.verificationID,
	}

	c.JSON(http.StatusOK, response.APIResponse[NINValidationResponse]{
		Status:  "success",
		Message: "NIN verification was successful.",
		Data:    resp,
	})
}

func (h *Handler) VerifyNewDevice(c *gin.Context) {
	var req NewDeviceResquest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	ip := c.ClientIP()

	authObj, err := h.service.VerifyNewDevice(c.Request.Context(), ip, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if authObj == nil || authObj.AccessToken == "" || authObj.RefreshToken == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Something went wrong.",
			},
		})
		return
	}

	resp := VerifiedDeviceResponse{
		AccessToken:         authObj.AccessToken,
		RefreshToken:        authObj.RefreshToken,
		IsBiometricsEnabled: authObj.IsBiometricsEnabled,
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "New device successfully verified.",
		Data:    &resp,
	})
}

func (h *Handler) ResendNewDeviceOTP(c *gin.Context) {
	var req ResendNewDeviceOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	if err := h.service.ResendNewDeviceOTP(c.Request.Context(), req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP successfully resent.",
	})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized",
			},
		})
		return
	}

	resp, err := h.service.ForgotPassword(c.Request.Context(), req, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ForgotPasswordResponse]{
		Status:  "success",
		Message: "OTP has been sent.",
		Data:    resp,
	})
}

func (h *Handler) ResendForgotPasswordOTP(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.ResendForgotPasswordOTP(c.Request.Context(), req, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ForgotPasswordResponse]{
		Status:  "success",
		Message: "OTP has been resent.",
		Data:    resp,
	})
}

func (h *Handler) VerifyForgotPasswordOTP(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req VerifyForgotPasswordOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	resp, err := h.service.VerifyForgotPasswordOTP(c.Request.Context(), deviceID, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifyForgotPasswordOTPResponse]{
		Status:  "success",
		Message: "OTP successfully verified.",
		Data:    resp,
	})
}

func (h *Handler) ForgotTransactionPin(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.ForgotTransactionPin(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ForgotTransactionPinResponse]{
		Status:  "success",
		Message: "OTP has been sent.",
		Data:    resp,
	})
}

func (h *Handler) VerifyForgotTransactionPinOTP(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req VerifyForgotTransactionPinOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	resp, err := h.service.VerifyForgotTransactionPinOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifyForgotTransactionPinOTPResponse]{
		Status:  "success",
		Message: "OTP has been verified.",
		Data:    resp,
	})
}

func (h *Handler) ResendForgotTransactionPinOTP(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	if err := h.service.ResendForgotTransactionPinOTP(c.Request.Context(), mobileUserID, deviceID); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP has been resent.",
	})
}

func (h *Handler) ResetTransactionPin(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req ResetTransactionPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	if err := h.service.ResetTransactionPin(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP has been sent.",
	})
}

func (h *Handler) RequestPasswordChange(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.RequestPasswordChange(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[RequestChangePasswordResponse]{
		Status:  "success",
		Message: "OTP has been sent.",
		Data:    resp,
	})
}

func (h *Handler) ResendPasswordChangeOTP(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.ResendPasswordChangeOTP(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) VerifyPasswordChangeOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req VerifyPasswordChangeOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	resp, err := h.service.VerifyPasswordChangeOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifyPasswordChangeOTPResponse]{
		Status:  "success",
		Message: "OTP successfully verified.",
		Data:    resp,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "Password has been changed successfully",
	})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	deviceID := c.Request.Header.Get("X-Device-ID")
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized.",
			},
		})
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req, deviceID); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "Password reset successfully.",
	})

}

func (h *Handler) RequestTransactionPinChange(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.RequestTransactionPinChange(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[RequestTransactionPinChangeResponse]{
		Status:  "success",
		Message: "OTP has been sent.",
		Data:    resp,
	})
}

func (h *Handler) ResendRequestTransactionPinChangeOTP(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.ResendTransactionPinChangeOTP(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[ResendTransactionPinChangeOTPResponse]{
		Status:  "success",
		Message: "OTP has been resent successfully.",
		Data:    resp,
	})
}

func (h *Handler) VerifyTransactionPinChangeOTP(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
			},
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req VerifyTransactionPinChangeOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	resp, err := h.service.VerifyTransactionPinChangeOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifyTransactionPinChangeOTPResponse]{
		Status:  "success",
		Message: "OTP has been verified.",
		Data:    resp,
	})
}

func (h *Handler) ChangeTransactionPin(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Unauthorized.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req ChangeTransactionPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	if err := h.service.ChangeTransactionPin(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "Transaction pin has been changed.",
	})
}

func (h *Handler) ToggleBiometrics(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidToken),
				Message: "Invalid request body.",
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
				Message: "Unauthorized.",
			},
		})
		return
	}

	resp, err := h.service.ToggleBiometrics(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ToggleBiometricsResponse]{
		Status:  "success",
		Message: "Biometrics toggled successfully.",
		Data:    resp,
	})
}

func (h *Handler) ChallengeRequest(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidDeviceID),
				Message: "Unauthorized.",
			},
		})
		return
	}

	var req ChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    string(ErrCodeInvalidRequestBody),
				Message: "Invalid request body.",
			},
		})
		return
	}

	resp, err := h.service.CreateChallenge(c.Request.Context(), strings.TrimSpace(req.RefreshToken), deviceID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[ChallengeRequestResponse]{
		Status:  "success",
		Message: "Challenge signature sent.",
		Data:    resp,
	})
}
