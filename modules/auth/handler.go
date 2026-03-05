package auth

import (
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

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()
	deviceID := c.GetHeader("X-Device-ID")

	tokenObj, err := h.service.Login(c.Request.Context(), deviceID, ip, userAgent, req.Email, req.Password)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{AccessToken: tokenObj.AccessToken, RefreshToken: tokenObj.RefreshToken})
}

func (h *Handler) Logout(c *gin.Context) {
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

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken, accessToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

func (h *Handler) RefreshAccessToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token missing"})
	}
}

func (h *Handler) VerifyBVN(c *gin.Context) {
	var req BVNValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bvn is missing"})
		return
	}

	bvnInfo, err := h.service.ValidateBVN(c.Request.Context(), req.BVN)
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

	const tendarStatusPrefix = "tendar bvn validation failed with status "
	if strings.HasPrefix(msg, tendarStatusPrefix) {
		statusCodeText := strings.TrimSpace(strings.TrimPrefix(msg, tendarStatusPrefix))
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

func (h *Handler) VerifyNIN(c *gin.Context) {
	var req NINValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nin is missing"})
		return
	}

	ninInfo, err := h.service.ValidateNIN(c.Request.Context(), req.NIN)
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
