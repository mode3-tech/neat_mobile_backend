package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *AuthService
}

func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		fmt.Println(err.Error())
		return
	}

	authObj, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		if isBadRequestRegisterError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isConflictRegisterError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "account or device already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
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
		"invalid Nigerian number":
		return true
	}

	return false
}

func isConflictRegisterError(err error) bool {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "duplicate key value violates unique constraint")
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()
	deviceID := c.GetHeader("X-Device-ID")

	loginObj, err := h.authService.Login(c.Request.Context(), deviceID, ip, userAgent, req.Phone, req.Password)

	if err != nil {
		if isBadRequestLoginError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isUnauthorizedLoginError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	if loginObj == nil || loginObj.Status == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	resp := LoginInitResponse{
		Status:       loginObj.Status,
		Challenge:    loginObj.Challenge,
		SessionToken: loginObj.SessionToken,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) VerifyDevice(c *gin.Context) {
	var req VerifyDeviceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	authObj, err := h.authService.VerifyDeviceChallenge(c.Request.Context(), req.Challenge, req.Signature, req.DeviceID, ip, userAgent)
	if err != nil {
		if isBadRequestVerifyDeviceError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isUnauthorizedVerifyDeviceError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	if authObj == nil || authObj.AccessToken == "" || authObj.RefreshToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
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

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization header"})
		return
	}
	splittedAuthHeader := strings.Fields(authHeader)
	if len(splittedAuthHeader) != 2 || splittedAuthHeader[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
		return
	}
	accessToken := splittedAuthHeader[1]

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken, accessToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

func (h *AuthHandler) RefreshAccessToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token missing"})
		return
	}

	tokenObj, err := h.authService.RefreshAccessToken(c.Request.Context(), strings.TrimSpace(req.RefreshToken))
	if err != nil {
		if isUnauthorizedRefreshError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	if tokenObj == nil || tokenObj.AccessToken == "" || tokenObj.RefreshToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{AccessToken: tokenObj.AccessToken, RefreshToken: tokenObj.RefreshToken})
}

func isUnauthorizedRefreshError(err error) bool {
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "invalid refresh token", "refresh token not found", "refresh token already revoked", "refresh token expired":
		return true
	}

	return false
}

func (h *AuthHandler) VerifyBVN(c *gin.Context) {
	var req BVNValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bvn is missing"})
		return
	}

	bvnInfo, err := h.authService.ValidateBVN(c.Request.Context(), req.BVN)
	if err != nil {
		if isBadRequestBVNError(err) {
			switch err.Error() {
			case "tendar bvn validation failed with status 404":
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bvn"})
				return
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		if status, message, ok := classifyUpstreamError(err); ok {
			c.JSON(status, gin.H{"error": message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	if bvnInfo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
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

func (h *AuthHandler) VerifyNIN(c *gin.Context) {
	var req NINValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nin is missing"})
		return
	}

	ninInfo, err := h.authService.ValidateNIN(c.Request.Context(), req.NIN)
	if err != nil {
		if isBadRequestNINError(err) {
			switch err.Error() {
			case "prembly nin validation failed with status 404":
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid nin"})
				return
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		if status, message, ok := classifyUpstreamError(err); ok {
			c.JSON(status, gin.H{"error": message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	if ninInfo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	c.JSON(http.StatusOK, &NINValidationResponse{
		Name:           ninInfo.name,
		DOB:            ninInfo.dob,
		PhoneNumber:    ninInfo.phone,
		VerificationID: ninInfo.verificationID,
	})
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
