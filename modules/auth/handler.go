package auth

import (
	"errors"
	"log"
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

func requestIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}

	if value, ok := c.Get(middleware.RequestIDContextKey); ok {
		if requestID, ok := value.(string); ok {
			requestID = strings.TrimSpace(requestID)
			if requestID != "" {
				return requestID
			}
		}
	}

	return strings.TrimSpace(c.GetHeader(middleware.RequestIDHeader))
}

func (h *Handler) respondError(c *gin.Context, status int, clientMessage string, err error) {
	if c == nil {
		return
	}

	requestID := requestIDFromContext(c)
	route := c.FullPath()
	if strings.TrimSpace(route) == "" {
		route = c.Request.URL.Path
	}

	level := "WARN"
	if status >= http.StatusInternalServerError {
		level = "ERROR"
	}

	if err != nil {
		_ = c.Error(err)
		log.Printf(
			"level=%s request_id=%s method=%s path=%s status=%d client_ip=%s error=%q",
			level,
			requestID,
			c.Request.Method,
			route,
			status,
			c.ClientIP(),
			err.Error(),
		)
	} else {
		log.Printf(
			"level=%s request_id=%s method=%s path=%s status=%d client_ip=%s error=%q",
			level,
			requestID,
			c.Request.Method,
			route,
			status,
			c.ClientIP(),
			clientMessage,
		)
	}

	c.AbortWithStatusJSON(status, gin.H{
		"error":      clientMessage,
		"request_id": requestID,
	})
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
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
		Message: "Registration processed successfully",
		Data:    resp,
	})
}

func (h *Handler) GetRegistrationStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		h.respondError(c, http.StatusBadRequest, "job id is required", nil)
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
		Message: "Registration status retrieved successfully",
		Data:    resp,
	})
}

func (h *Handler) ClaimRegistrationSession(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	if jobID == "" {
		h.respondError(c, http.StatusBadRequest, "job id is required", nil)
		return
	}

	var req RegistrationSessionClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty registration session claim response"))
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "Registration session claimed successfully",
		Data:    resp,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	ip := c.ClientIP()
	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty login init response"))
		return
	}

	resp := LoginInitResponse{
		Challenge:    loginObj.Challenge,
		SessionToken: loginObj.SessionToken,
	}

	c.JSON(http.StatusOK, response.APIResponse[LoginInitResponse]{
		Status:  "success",
		Message: "login credentials verified",
		Data:    &resp,
	})
}

func (h *Handler) VerifyDevice(c *gin.Context) {
	var req VerifyDeviceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty verify-device response"))
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "device successfully verified",
		Data:    resp,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		h.respondError(c, http.StatusBadRequest, "missing authorization header", nil)
		return
	}
	splittedAuthHeader := strings.Fields(authHeader)
	if len(splittedAuthHeader) != 2 || splittedAuthHeader[0] != "Bearer" {
		h.respondError(c, http.StatusUnauthorized, "invalid authorization header", nil)
		return
	}
	accessToken := splittedAuthHeader[1]

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken, accessToken); err != nil {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	c.JSON(http.StatusNoContent, response.APIResponse[any]{
		Status:  "success",
		Message: "logout successful",
	})
}

func (h *Handler) RefreshAccessToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "refresh token or device id missing", err)
		return
	}

	tokenObj, err := h.service.RefreshAccessToken(c.Request.Context(), strings.TrimSpace(req.DeviceID), strings.TrimSpace(req.RefreshToken))
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	if tokenObj == nil || tokenObj.AccessToken == "" || tokenObj.RefreshToken == "" {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty token response"))
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
		h.respondError(c, http.StatusBadRequest, "bvn is missing", err)
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty bvn response"))
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
		Message: "bvn verification was succesful",
		Data:    resp,
	})
}

func (h *Handler) VerifyNIN(c *gin.Context) {
	var req NINValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "nin is missing", err)
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty nin response"))
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
		Message: "nin verification was successful",
		Data:    resp,
	})
}

func (h *Handler) VerifyNewDevice(c *gin.Context) {
	var req NewDeviceResquest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
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
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty verify-device response"))
		return
	}

	resp := VerifiedDeviceResponse{
		AccessToken:         authObj.AccessToken,
		RefreshToken:        authObj.RefreshToken,
		IsBiometricsEnabled: authObj.IsBiometricsEnabled,
	}

	c.JSON(http.StatusOK, response.APIResponse[VerifiedDeviceResponse]{
		Status:  "success",
		Message: "new device successfully verified",
		Data:    &resp,
	})
}

func (h *Handler) ResendNewDeviceOTP(c *gin.Context) {
	var req ResendNewDeviceOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
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
		Message: "otp successfully resent",
	})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error: &response.APIError{
				Code:    "MISSING_DEVICE_ID",
				Message: "unauthorized",
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
		Message: "otp has been sent",
		Data:    resp,
	})
}

func (h *Handler) ResendForgotPasswordOTP(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	deviceID := c.Request.Header.Get("X-Device-ID")

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
		Message: "otp has been resent",
		Data:    resp,
	})
}

func (h *Handler) VerifyForgotPasswordOTP(c *gin.Context) {
	deviceID := c.Request.Header.Get("X-Device-ID")

	var req VerifyForgotPasswordOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
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
		Message: "verification of otp was successful",
		Data:    resp,
	})
}

func (h *Handler) ForgotTransactionPin(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	resp, err := h.service.ForgotTransactionPin(c.Request.Context(), mobileUserID, deviceID)

	if err != nil {
		if isForgotPinBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isForgotPinUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isForgotPinBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "invalid phone number on account":
		return true
	}
	return false
}

func isForgotPinUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func (h *Handler) VerifyForgotTransactionPinOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req VerifyForgotTransactionPinOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	resp, err := h.service.VerifyForgotTransactionPinOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		if isVerifyForgotTransactionPinOTPBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isVerifyForgotTransactionPinOTPUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isVerifyForgotTransactionPinOTPBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "otp id is required", "otp code is required":
		return true
	}
	return false
}

func isVerifyForgotTransactionPinOTPUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed", "user not found", "invalid otp":
		return true
	}
	return false
}

func (h *Handler) ResendForgotTransactionPinOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	if err := h.service.ResendForgotTransactionPinOTP(c.Request.Context(), mobileUserID, deviceID); err != nil {
		if isResendForgotPinBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isResendForgotPinUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		if isResendForgotPinRateLimitedError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP resent successfully"})
}

func isResendForgotPinBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required":
		return true
	}
	return false
}

func isResendForgotPinUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func isResendForgotPinRateLimitedError(err error) bool {
	return strings.TrimSpace(err.Error()) == "too many requests"
}

func (h *Handler) ResetTransactionPin(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req ResetTransactionPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.ResetTransactionPin(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		if isResetPinBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isResetPinUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transaction pin reset successfully"})
}

func isResetPinBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required",
		"otp code is required",
		"transaction pin must be exactly 4 digits long",
		"transaction pin must contain only digits",
		"new pin and confirm new pin do not match":
		return true
	}
	return false
}

func isResetPinUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid device id", "invalid otp", "user not found", "invalid verification id", "verification id has expired", "device not found":
		return true
	}
	return false
}

func (h *Handler) RequestPasswordChange(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	resp, err := h.service.RequestPasswordChange(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isRequestPasswordChangeBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isRequestPasswordChangeUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isRequestPasswordChangeBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "invalid phone number on account":
		return true
	}
	return false
}

func isRequestPasswordChangeUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func (h *Handler) ResendPasswordChangeOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	resp, err := h.service.ResendPasswordChangeOTP(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isResendPasswordChangeOTPBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isResendPasswordChangeOTPUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		if isResendPasswordChangeOTPRateLimitedError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isResendPasswordChangeOTPBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required":
		return true
	}
	return false
}

func isResendPasswordChangeOTPUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func isResendPasswordChangeOTPRateLimitedError(err error) bool {
	return strings.TrimSpace(err.Error()) == "too many requests"
}

func (h *Handler) VerifyPasswordChangeOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req VerifyPasswordChangeOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	resp, err := h.service.VerifyPasswordChangeOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		if isVerifyPasswordChangeOTPBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isVerifyPasswordChangeOTPUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isVerifyPasswordChangeOTPBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "otp id is required", "otp code is required":
		return true
	}
	return false
}

func isVerifyPasswordChangeOTPUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed", "user not found", "invalid otp":
		return true
	}
	return false
}

func (h *Handler) ChangePassword(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		if isChangePasswordBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isChangePasswordUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

func isChangePasswordBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required",
		"verification id is required",
		"password length should be at least 8 characters long",
		"password must contain at least one uppercase letter, one lowercase letter, one number, and one special character",
		"new password and confirm new password do not match":
		return true
	}
	return false
}

func isChangePasswordUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed", "user not found",
		"invalid current password",
		"invalid verification id", "verification id has expired":
		return true
	}
	return false
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	deviceID := c.Request.Header.Get("X-Device-ID")

	if err := h.service.ResetPassword(c.Request.Context(), req, deviceID); err != nil {
		if isBadRequestResetPasswordError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isUnauthorizedResetPasswordError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})

}

func isBadRequestResetPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required",
		"phone is required",
		"verification id is required",
		"password length should be at least 8 characters long",
		"password must contain at least one uppercase letter, one lowercase letter, one number, and one special character",
		"new password and confirm new password do not match",
		"invalid Nigerian number":
		return true
	}

	return false
}

func isUnauthorizedResetPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed",
		"invalid verification id", "verification id has expired",
		"no account exists under this phone number":
		return true
	}

	return false
}

func (h *Handler) RequestTransactionPinChange(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	resp, err := h.service.RequestTransactionPinChange(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isRequestTransactionPinChangeBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isRequestTransactionPinChangeUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		if isResendRequestTransactionPinChangeRateLimitedError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isRequestTransactionPinChangeBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "invalid phone number on account":
		return true
	}
	return false
}

func isRequestTransactionPinChangeUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func (h *Handler) ResendRequestTransactionPinChangeOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	resp, err := h.service.ResendTransactionPinChangeOTP(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isResendRequestTransactionPinChangeBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isResendRequestTransactionPinChangeUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		if isResendRequestTransactionPinChangeRateLimitedError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func isResendRequestTransactionPinChangeBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required":
		return true
	}
	return false
}

func isResendRequestTransactionPinChangeUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no record of device found", "user not found":
		return true
	}
	return false
}

func isResendRequestTransactionPinChangeRateLimitedError(err error) bool {
	return strings.TrimSpace(err.Error()) == "too many requests"
}

func (h *Handler) VerifyTransactionPinChangeOTP(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req VerifyTransactionPinChangeOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	resp, err := h.service.VerifyTransactionPinChangeOTP(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		if isVerifyTransactionPinChangeOTPBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isVerifyTransactionPinChangeOTPUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isVerifyTransactionPinChangeOTPBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "otp id is required", "otp code is required":
		return true
	}
	return false
}

func isVerifyTransactionPinChangeOTPUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed", "user not found", "invalid otp":
		return true
	}
	return false
}

func (h *Handler) ChangeTransactionPin(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if strings.TrimSpace(mobileUserID) == "" {
		h.respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	var req ChangeTransactionPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.ChangeTransactionPin(c.Request.Context(), mobileUserID, deviceID, req); err != nil {
		if isChangeTransactionPinBadRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isChangeTransactionPinUnauthorizedError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transaction pin successfully changed"})
}

func isChangeTransactionPinBadRequestError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required",
		"verification id is required",
		"transaction pin must be exactly 4 digits long",
		"transaction pin must contain only digits",
		"new pin and confirm new pin do not match",
		"invalid current pin":
		return true
	}
	return false
}

func isChangeTransactionPinUnauthorizedError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device not found", "device not allowed", "user not found",
		"invalid verification id", "verification id has expired":
		return true
	}
	return false
}

func (h *Handler) ToggleBiometrics(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		h.respondError(c, http.StatusUnauthorized, "invalid access token", nil)
		return
	}

	deviceID := c.GetHeader("X-Device-ID")

	resp, err := h.service.ToggleBiometrics(c.Request.Context(), mobileUserID, deviceID)
	if err != nil {
		if isBadRequestToggleBiometricsError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isUnprocessableEntityBiometricsError(err) {
			h.respondError(c, http.StatusUnprocessableEntity, err.Error(), err)
			return
		}
		if isUnauthorizedToggleBiometricsError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		if isInternalServerBiometricsError(err) {
			h.respondError(c, http.StatusInternalServerError, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isBadRequestToggleBiometricsError(err error) bool {
	msg := err.Error()
	switch msg {
	case "is_enabled must be true or false":
		return true
	}
	return false
}

func isUnauthorizedToggleBiometricsError(err error) bool {
	msg := err.Error()
	switch msg {
	case "device is not allowed", "device not found", "device id is required", "mobile user id is required":
		return true
	}

	return false
}

func isUnprocessableEntityBiometricsError(err error) bool {
	msg := err.Error()
	switch msg {
	case "is_enabled should be true or false":
		return true
	}
	return false
}

func isInternalServerBiometricsError(err error) bool {
	msg := err.Error()
	switch msg {
	case "unable to toggle biometrics":
		return true
	}
	return false
}

func (h *Handler) ChallengeRequest(c *gin.Context) {
	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		h.respondError(c, http.StatusBadRequest, "unauthorized", nil)
		return
	}

	var req ChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "refresh_token is required", err)
		return
	}

	resp, err := h.service.CreateChallenge(c.Request.Context(), strings.TrimSpace(req.RefreshToken), deviceID)
	if err != nil {
		if isBadRequestChallengeRequestError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isUnauthorizedChallengeRequestError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isBadRequestChallengeRequestError(err error) bool {
	msg := err.Error()
	switch msg {
	case "device id is required":
		return true
	}
	return false
}

func isUnauthorizedChallengeRequestError(err error) bool {
	msg := err.Error()
	switch msg {
	case "device not found", "device not allowed", "mobile user id is required":
		return true
	}
	return false
}
