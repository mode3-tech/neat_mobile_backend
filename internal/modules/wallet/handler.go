package wallet

import (
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
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

func (h *Handler) FetchBanks(c *gin.Context) {
	banks, err := h.service.FetchBanks(c.Request.Context())
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]Bank]{
		Status:  "success",
		Message: "Banks fetched successfully with sortcodes",
		Data:    &banks,
	})

}

func (h *Handler) FetchBankDetails(c *gin.Context) {
	var query BankDetailsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	bankDetails, err := h.service.FetchBankDetails(c.Request.Context(), query.AccountNumber, query.BankCode)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp := &BankDetails{
		BankCode:      bankDetails.BankCode,
		AccountName:   bankDetails.AccountName,
		AccountNumber: bankDetails.AccountNumber,
	}

	c.JSON(http.StatusOK, response.APIResponse[*BankDetails]{
		Status:  "success",
		Message: "Bank details fetched successfully",
		Data:    &resp,
	})
}

func (h *Handler) InitiateTransfer(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	transferResponse, err := h.service.InitiateTransfer(c.Request.Context(), mobileUserID, &req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	dto := &TransferResult{
		Amount:               transferResponse.Transfer.Amount,
		Charges:              transferResponse.Transfer.Charges,
		Vat:                  transferResponse.Transfer.Vat,
		Reference:            transferResponse.Transfer.Reference,
		Total:                transferResponse.Transfer.Total,
		Metadata:             transferResponse.Transfer.Metadata,
		SessionID:            transferResponse.Transfer.SessionID,
		Destination:          transferResponse.Transfer.Destination,
		TransactionReference: transferResponse.Transfer.TransactionReference,
		Description:          transferResponse.Transfer.Description,
	}

	c.JSON(http.StatusOK, response.APIResponse[*TransferResult]{
		Status:  "success",
		Message: "Transfer success",
		Data:    &dto,
	})
}

func (h *Handler) AddBeneficiary(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
	if deviceID == "" {
		mapped := response.MapError(appErr.ErrMissingDeviceID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var req AddBeneficiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	beneficiary, err := h.service.AddBeneficiary(c.Request.Context(), mobileUserID, &req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := &AddBeneficiaryResponse{
		Beneficiary: *beneficiary,
	}

	c.JSON(http.StatusOK, response.APIResponse[AddBeneficiaryResponse]{
		Status:  "success",
		Message: "Beneficiary added successfully",
		Data:    dto,
	})
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

	// var query FetchBeneficiariesQuery
	// if err := c.ShouldBindQuery(&query); err != nil {
	// 	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
	// 	return
	// }
	beneficiaries, err := h.service.GetBeneficiaries(c.Request.Context(), mobileUserID)

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
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrMissingUserID)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
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
			mapped := response.MapError(appErr.ErrInvalidRequestBody)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}
	}

	resp, err := h.service.InitiateBulkTransfer(c.Request.Context(), mobileUserID, &req)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
	}

	c.JSON(http.StatusOK, resp)
}
