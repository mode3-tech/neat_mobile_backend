package wallet

import (
	"log"
	"neat_mobile_app_backend/internal/middleware"
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

func (h *Handler) InitiateTransfer(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	transferResponse, err := h.service.InitiateTransfer(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error initiating transfer: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate transfer"})
		return
	}
	c.JSON(http.StatusOK, transferResponse)
}

func (h *Handler) AddBeneficiary(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req AddBeneficiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	beneficiary, err := h.service.AddBeneficiary(c.Request.Context(), mobileUserID, &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to add beneficiary"})
		return
	}

	response := &AddBeneficiaryResponse{
		Status:      true,
		Message:     "Beneficiary added successfully",
		Beneficiary: *beneficiary,
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetBeneficiaries(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var query FetchBeneficiariesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}
	beneficiaries, err := h.service.GetBeneficiaries(c.Request.Context(), mobileUserID, query.WalletID)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch beneficiaries"})
		return
	}

	response := &FetchBeneficiariesResponse{
		Status:        true,
		Message:       "Beneficiaries fetched successfully",
		Beneficiaries: beneficiaries,
	}

	c.JSON(http.StatusOK, response)
}
