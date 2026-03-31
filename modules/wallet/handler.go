package wallet

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) FetchBanks(c *gin.Context) {
	// userID := middleware.UserIDContextKey

	// if strings.TrimSpace(userID) == "" {
	// 	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	// 	return
	// }

	banks, err := h.service.FetchBanks(c.Request.Context())

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "banks": banks})

}
