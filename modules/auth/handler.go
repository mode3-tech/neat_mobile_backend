package auth

import (
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

	token, err := h.service.Login(c.Request.Context(), req.Email, req.Password)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{AccessToken: token})
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
