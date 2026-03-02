package auth

import (
	"fmt"
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": err})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
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

func (h *Handler) SendOTP(c *gin.Context) {
	var req SMSOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phone number is missing"})
	}

}

func (h *Handler) VerifyBVN(c *gin.Context) {
	var req BVNValidationRequest

	selectedBVNService := BVNServiceType(c.Query("service"))

	fmt.Println(selectedBVNService)

	if selectedBVNService != TendarServiceType && selectedBVNService != PremblyServiceType {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service must be 'tendar' or 'prembly'"})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bvn is missing"})
		return
	}

	switch selectedBVNService {
	case TendarServiceType:
		bvnInfo, err := h.service.ValidateBVNWithTendar(c.Request.Context(), req.BVN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, &BVNValidationResponse{
			Name:        bvnInfo.name,
			DOB:         bvnInfo.dob,
			PhoneNumber: bvnInfo.phone,
		})
	case PremblyServiceType:
		bvnInfo, err := h.service.ValidateBVNWithPrembly(c.Request.Context(), req.BVN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, &BVNValidationResponse{
			Name:        bvnInfo.name,
			DOB:         bvnInfo.dob,
			PhoneNumber: bvnInfo.phone,
		})
	}
}
