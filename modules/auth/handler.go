package auth

import (
	"context"
	"errors"
	"log"
	"neat_mobile_app_backend/internal/middleware"
	"net"
	"net/http"
	"strconv"
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
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	ip := c.ClientIP()

	authObj, err := h.service.Register(c.Request.Context(), req, ip)
	if err != nil {
		if isBadRequestRegisterError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isConflictRegisterError(err) {
			h.respondError(c, http.StatusConflict, "account or device already exists", err)
			return
		}
		if isBadPasswordError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}

		if status, message, ok := classifyUpstreamError(err); ok {
			h.respondError(c, status, message, err)
			return
		}
		if isProvidusWalletError(err) {
			h.respondError(c, http.StatusBadGateway, err.Error(), err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "registration successful", "access_token": authObj.AccessToken, "refresh_token": authObj.RefreshToken})
}

func isBadRequestRegisterError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "phone verification record not found",
		"bvn verification record not found",
		"nin verification record not found",
		"email verification record not found",
		"unable to confirm email and phone number belong to the same person due to names or date of births mismatch",
		"unable to confirm bvn and nin belong to the same person due to names or date of births mismatch",
		"passwords do not match",
		"transaction pins do not match",
		"device not found",
		"invalid Nigerian number":
		return true
	}

	return false
}

func isConflictRegisterError(err error) bool {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch msg {
	case "user already exists",
		"device already exists",
		"bvn already linked to another user":
		return true
	default:
		return strings.Contains(msg, "duplicate key value violates unique constraint")

	}
}

func isBadPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	return msg == "password length should be at least 8 characters long" ||
		msg == "password must contain at least one uppercase letter, one lowercase letter, one number, and one special character"
}

func isProvidusWalletError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	return strings.HasPrefix(msg, "providus wallet") ||
		strings.HasPrefix(msg, "failed to decode providus wallet generation")
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	ip := c.ClientIP()
	deviceID := c.GetHeader("X-Device-ID")

	loginObj, err := h.service.Login(c.Request.Context(), deviceID, ip, req.Phone, req.Password)

	if err != nil {
		if isBadRequestLoginError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isRateLimitedLoginError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		if isUnauthorizedLoginError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if loginObj == nil || loginObj.Status == "" {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty login init response"))
		return
	}

	resp := LoginInitResponse{
		Status:       loginObj.Status,
		Challenge:    loginObj.Challenge,
		SessionToken: loginObj.SessionToken,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) VerifyDevice(c *gin.Context) {
	var req VerifyDeviceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	ip := c.ClientIP()

	authObj, err := h.service.VerifyDeviceChallenge(c.Request.Context(), req.Challenge, req.Signature, req.DeviceID, ip)
	if err != nil {
		if isBadRequestVerifyDeviceError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isUnauthorizedVerifyDeviceError(err) {
			h.respondError(c, http.StatusUnauthorized, err.Error(), err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if authObj == nil || authObj.AccessToken == "" || authObj.RefreshToken == "" {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty verify-device response"))
		return
	}

	c.JSON(http.StatusOK, VerifyDeviceResponse{
		Status:       "success",
		AccessToken:  authObj.AccessToken,
		RefreshToken: authObj.RefreshToken,
	})
}

func isBadRequestLoginError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required", "invalid Nigerian number":
		return true
	}

	return false
}

func isUnauthorizedLoginError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid credentials":
		return true
	}

	return false
}

func isRateLimitedLoginError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	return msg == "too many requests"
}

func isBadRequestVerifyDeviceError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "challenge is required", "signature is required", "device id is required":
		return true
	}

	return false
}

func isUnauthorizedVerifyDeviceError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid challenge", "device verification failed":
		return true
	}

	return false
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
	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

func (h *Handler) RefreshAccessToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "refresh token or device id missing", err)
		return
	}

	tokenObj, err := h.service.RefreshAccessToken(c.Request.Context(), strings.TrimSpace(req.DeviceID), strings.TrimSpace(req.RefreshToken))
	if err != nil {
		if isBadRequestRefreshError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isUnauthorizedRefreshError(err) {
			h.respondError(c, http.StatusUnauthorized, "invalid refresh token", err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if tokenObj == nil || tokenObj.AccessToken == "" || tokenObj.RefreshToken == "" {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty token response"))
		return
	}

	c.JSON(http.StatusOK, LoginResponse{AccessToken: tokenObj.AccessToken, RefreshToken: tokenObj.RefreshToken})
}

func isBadRequestRefreshError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "device id is required":
		return true
	}

	return false
}

func isUnauthorizedRefreshError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid refresh token", "refresh token not found", "refresh token already revoked", "refresh token expired", "device not found", "device not allowed", "invalid session":
		return true
	}

	return false
}

func (h *Handler) VerifyBVN(c *gin.Context) {
	var req BVNValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "bvn is missing", err)
		return
	}

	bvnInfo, err := h.service.ValidateBVN(c.Request.Context(), req.BVN)
	if err != nil {
		if isBadRequestBVNError(err) {
			switch err.Error() {
			case "tendar bvn validation failed with status 404":
				h.respondError(c, http.StatusBadRequest, "invalid bvn", err)
				return
			default:
				h.respondError(c, http.StatusBadRequest, err.Error(), err)
				return
			}
		}
		if status, message, ok := classifyUpstreamError(err); ok {
			h.respondError(c, status, message, err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if bvnInfo == nil {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty bvn response"))
		return
	}

	c.JSON(http.StatusOK, &BVNValidationResponse{
		Name:           bvnInfo.name,
		DOB:            bvnInfo.dob,
		PhoneNumber:    bvnInfo.phone,
		VerificationID: bvnInfo.verificationID,
	})
}

func isBadRequestBVNError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "bvn is required", "invalid bvn number":
		return true
	}

	prefixes := []string{
		"tendar bvn validation failed with status ",
		"prembly bvn validation failed with status ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(msg, prefix) {
			statusCodeText := strings.TrimSpace(strings.TrimPrefix(msg, prefix))
			statusCode, convErr := strconv.Atoi(statusCodeText)
			if convErr == nil {
				switch statusCode {
				case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
					return true
				}
			}
		}
	}

	return false
}

func (h *Handler) VerifyNIN(c *gin.Context) {
	var req NINValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "nin is missing", err)
		return
	}

	ninInfo, err := h.service.ValidateNIN(c.Request.Context(), req.BVNValidationID, req.NIN)
	if err != nil {
		if isBadRequestNINError(err) {
			switch err.Error() {
			case "prembly nin validation failed with status 404":
				h.respondError(c, http.StatusBadRequest, "invalid nin", err)
				return
			default:
				h.respondError(c, http.StatusBadRequest, err.Error(), err)
				return
			}
		}

		if isBVNAndNINNotAMatch(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}

		if status, message, ok := classifyUpstreamError(err); ok {
			h.respondError(c, status, message, err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if ninInfo == nil {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty nin response"))
		return
	}

	c.JSON(http.StatusOK, &NINValidationResponse{
		Name:           ninInfo.name,
		DOB:            ninInfo.dob,
		PhoneNumber:    ninInfo.phone,
		VerificationID: ninInfo.verificationID,
	})
}

func isBVNAndNINNotAMatch(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "bvn name does not match nin name", "bvn dob does not match nin dob":
		return true
	default:
		return false
	}
}

func isBadRequestNINError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "nin is required", "invalid nin", "invalid nin number":
		return true
	}

	const premblyStatusPrefix = "prembly nin validation failed with status "
	if strings.HasPrefix(msg, premblyStatusPrefix) {
		statusCodeText := strings.TrimSpace(strings.TrimPrefix(msg, premblyStatusPrefix))
		statusCode, convErr := strconv.Atoi(statusCodeText)
		if convErr == nil {
			switch statusCode {
			case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
				return true
			}
		}
	}

	return false
}

func classifyUpstreamError(err error) (int, string, bool) {
	if err == nil {
		return 0, "", false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "upstream service timed out, please try again", true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return http.StatusGatewayTimeout, "upstream service timed out, please try again", true
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout") {
		return http.StatusGatewayTimeout, "upstream service timed out, please try again", true
	}

	statusCode, ok := extractUpstreamStatusCode(msg)
	if !ok {
		return 0, "", false
	}

	switch {
	case statusCode >= http.StatusInternalServerError:
		return http.StatusServiceUnavailable, "upstream service unavailable, please try again", true
	case statusCode == http.StatusTooManyRequests:
		return http.StatusServiceUnavailable, "upstream service unavailable, please try again", true
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return http.StatusServiceUnavailable, "upstream service unavailable, please try again", true
	default:
		return 0, "", false
	}
}

func extractUpstreamStatusCode(msg string) (int, bool) {
	prefixes := []string{
		"tendar bvn validation failed with status ",
		"prembly bvn validation failed with status ",
		"prembly nin validation failed with status ",
		"cba returned status ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(msg, prefix) {
			statusCodeText := strings.TrimSpace(strings.TrimPrefix(msg, prefix))
			statusCode, err := strconv.Atoi(statusCodeText)
			if err == nil {
				return statusCode, true
			}
		}
	}

	return 0, false
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
		if isBadRequestVerifyNewDeviceError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}

		if isUnauthorizedVerifyNewDeviceError(err) {
			h.respondError(c, http.StatusUnauthorized, "expired session", err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	if authObj == nil || authObj.AccessToken == "" || authObj.RefreshToken == "" {
		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", errors.New("empty verify-device response"))
		return
	}

	c.JSON(http.StatusOK, VerifyDeviceResponse{
		Status:       "success",
		AccessToken:  authObj.AccessToken,
		RefreshToken: authObj.RefreshToken,
	})
}

func isBadRequestVerifyNewDeviceError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "otp is required", "session token is required", "device id is required", "public key is required":
		return true
	}
	return false
}

func isUnauthorizedVerifyNewDeviceError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "invalid session token", "invalid otp":
		return true
	}

	return false
}

func (h *Handler) ResendNewDeviceOTP(c *gin.Context) {
	var req ResendNewDeviceOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.ResendNewDeviceOTP(c.Request.Context(), req); err != nil {
		if isBadRequestResendNewDeviceOTPError(err) {
			h.respondError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		if isRateLimitedResendNewDeviceOTPError(err) {
			h.respondError(c, http.StatusTooManyRequests, err.Error(), err)
			return
		}
		if isUnauthorizedResendNewDeviceOTPError(err) {
			h.respondError(c, http.StatusUnauthorized, "expired session", err)
			return
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "otp resent successfully"})
}

func isBadRequestResendNewDeviceOTPError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "session token is required", "device id is required":
		return true
	}
	return false
}

func isRateLimitedResendNewDeviceOTPError(err error) bool {
	return strings.TrimSpace(err.Error()) == "too many requests"
}

func isUnauthorizedResendNewDeviceOTPError(err error) bool {
	return strings.TrimSpace(err.Error()) == "invalid session token"
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid request body", err)
	}

	deviceID := c.Request.Header.Get("X-Device-ID")

	if err := h.service.ForgotPassword(c.Request.Context(), req, deviceID); err != nil {
		if isNoUserForgotPasswordError(err) {
			switch err.Error() {
			case "no account exists under this phone number":
				h.respondError(c, http.StatusUnauthorized, err.Error(), err)
				return
			default:
				h.respondError(c, http.StatusUnauthorized, "no account exists under this phone number", err)
				return
			}
		}
		if isOTPForgotPasswordError(err) {
			switch err.Error() {
			case "error occured while saving otp":
				h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
				return
			}
		}

		h.respondError(c, http.StatusInternalServerError, "something went wrong, please try again later", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Otp has been sent your phone as sms"})

}

func isNoUserForgotPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no account exists under this phone number":
		return true
	case "no record of device found":
		return true
	}
	return false
}

func isOTPForgotPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "error occured while saving otp":
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
		"reset code is required",
		"password length should be at least 8 characters long",
		"password must contain at least one uppercase letter, one lowercase letter, one number, and one special character":
		return true
	}

	return false
}

func isUnauthorizedResetPasswordError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid device id", "invalid reset code", "no account exists under this phone number":
		return true
	}

	return false
}
