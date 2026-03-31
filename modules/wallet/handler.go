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

func (h *Handler) FetchBankDetails(c *gin.Context) {
	var query BankDetailsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	bankDetails, err := h.service.FetchBankDetails(c.Request.Context(), query.AccountNumber, query.BankCode)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bank details"})
		return
	}

	response := &BankDetailsResponse{
		Status:  true,
		Account: *bankDetails,
	}

	c.JSON(http.StatusOK, response)
}
