package wallet

import (
	"errors"
	"log"
	"neat_mobile_app_backend/internal/middleware"
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

func (h *Handler) FetchBanks(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)

	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	banks, err := h.service.FetchBanks(c.Request.Context(), mobileUserID, deviceID)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "banks": banks})

}

func (h *Handler) FetchBankDetails(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)

	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	var query BankDetailsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	bankDetails, err := h.service.FetchBankDetails(c.Request.Context(), query.AccountNumber, query.BankCode, mobileUserID, deviceID)
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
	mobileUserID := c.GetString(middleware.UserIDContextKey)

	if strings.TrimSpace(mobileUserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	transferResponse, err := h.service.InitiateTransfer(c.Request.Context(), mobileUserID, deviceID, &req)
	if err != nil {
		h.handleInitiateTransferError(c, err)
		return
	}
	c.JSON(http.StatusOK, transferResponse)
}

func (h *Handler) handleInitiateTransferError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWrongTransactionPin):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTransactionPinLocked):
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Transaction PIN is locked due to too many failed attempts. Try again later"})
	case errors.Is(err, ErrInvalidTransferRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrDeviceVerificationFailed):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device verification failed"})
	case errors.Is(err, ErrWalletNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
	case errors.Is(err, ErrTransferProviderFailed):
		log.Printf("Transfer provider error: %v", err)
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		log.Printf("Error initiating transfer: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate transfer"})
	}
}

func (h *Handler) AddBeneficiary(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	var req AddBeneficiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	beneficiary, err := h.service.AddBeneficiary(c.Request.Context(), mobileUserID, deviceID, &req)
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

func (h *Handler) HandleCreditWebhook(c *gin.Context) {
	var payload ProvidusCredit
	if err := c.ShouldBindJSON(&payload); err != nil {
		// Always return 200 — a non-200 causes Providus to retry indefinitely
		log.Printf("providus credit webhook: invalid payload: %v", err)
		c.JSON(http.StatusOK, gin.H{"status": true})
		return
	}

	if err := h.service.HandleCreditWebhook(c.Request.Context(), &payload); err != nil {
		log.Printf("providus credit webhook: processing error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": true})
}

func (h *Handler) GetBeneficiaries(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	// var query FetchBeneficiariesQuery
	// if err := c.ShouldBindQuery(&query); err != nil {
	// 	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
	// 	return
	// }
	beneficiaries, err := h.service.GetBeneficiaries(c.Request.Context(), mobileUserID, deviceID)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch beneficiaries"})
		return
	}

	result := make([]BeneficiaryResponseStruct, len(beneficiaries))
	for i, b := range beneficiaries {
		result[i] = BeneficiaryResponseStruct{
			WalletID:      b.WalletID,
			BankCode:      b.BankCode,
			AccountNumber: b.AccountNumber,
			AccountName:   b.AccountName,
		}
	}

	response := &FetchBeneficiariesResponse{
		Status:        true,
		Message:       "Beneficiaries fetched successfully",
		Beneficiaries: result,
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) InitiateBulkTransfer(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if strings.TrimSpace(deviceID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device ID is required"})
		return
	}

	var req BulkTransferRequest

	if strings.Contains(c.ContentType(), "multipart/form-data") {
		pin := c.PostForm("transaction_pin")
		if strings.TrimSpace(pin) == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "transaction_pin is required"})
			return
		}

		fileHeader, err := c.FormFile("recipients_excel")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "recipients_excel file is required"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
			return
		}
		defer file.Close()

		recipients, err := parseExcel(file)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		req = BulkTransferRequest{
			RecipientInfo:  recipients,
			TransactionPin: pin,
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}

	resp, err := h.service.InitiateBulkTransfer(c.Request.Context(), mobileUserID, deviceID, &req)
	if err != nil {
		h.handleBulkTransferError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) handleBulkTransferError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWrongTransactionPin):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid transaction PIN"})
	case errors.Is(err, ErrTransactionPinLocked):
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Transaction PIN is locked due to too many failed attempts. Try again later"})
	case errors.Is(err, ErrInvalidTransferRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "One or more transfer requests are invalid"})
	case errors.Is(err, ErrDeviceVerificationFailed):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Device verification failed"})
	case errors.Is(err, ErrWalletNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
	case errors.Is(err, ErrTransferProviderFailed):
		log.Printf("Bulk transfer provider error: %v", err)
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "Transfer service is temporarily unavailable. Please try again later"})
	default:
		log.Printf("Error initiating bulk transfer: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate bulk transfer"})
	}
}
