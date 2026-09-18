package wallet

import (
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	apiKey  string
}

func NewHandler(service *Service, apiKey string) *Handler {
	return &Handler{service: service, apiKey: apiKey}
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
		mapped := response.MapError(appErr.ErrUnauthorized)
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
		return
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

// func (h *Handler) HandleCreditWebhook(c *gin.Context) {

// 	rawBody, err := c.GetRawData()
// 	if err != nil {
// 		log.Printf("providus credit webhook: error: %v", err)
// 		mapped := response.MapError(appErr.ErrBadRequest)
// 		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
// 			Status: "error",
// 			Error:  &mapped.Error,
// 		})
// 		return
// 	}

// 	signature := c.Request.Header.Get("x-xpresswallet-signature")
// 	log.Printf("providus credit webhook: signature: %s", signature)
// 	if signature == "" {
// 		log.Printf("providus credit webhook: error: signature is empty")
// 		mapped := response.MapError(appErr.ErrUnauthorized)
// 		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
// 			Status: "error",
// 			Error:  &mapped.Error,
// 		})
// 		return
// 	}

// 	if !verifySignature(rawBody, signature, h.apiKey) {
// 		log.Printf("providus credit webhook: error: invalid signature")
// 		mapped := response.MapError(appErr.ErrUnauthorized)
// 		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
// 			Status: "error",
// 			Error:  &mapped.Error,
// 		})
// 		return
// 	}

// 	var event XpressWalletEvent
// 	if err := json.Unmarshal(rawBody, &event); err != nil {
// 		log.Printf("providus credit webhook: error: %v", err)
// 		mapped := response.MapError(appErr.ErrBadRequest)
// 		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
// 			Status: "error",
// 			Error:  &mapped.Error,
// 		})
// 		return
// 	}

// 	if err := h.service.ProcessCreditWebhook(c.Request.Context(), event); err != nil {
// 		log.Printf("providus credit webhook: error: %v", err)
// 		mapped := response.MapError(err)
// 		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
// 			Status: "error",
// 			Error:  &mapped.Error,
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": true})
// }

func (h *Handler) GetBeneficiaries(c *gin.Context) {
	mobileUserID := c.GetString(middleware.UserIDContextKey)
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	beneficiaries, err := h.service.GetBeneficiaries(c.Request.Context(), mobileUserID)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch beneficiaries"})
		return
	}

	result := make([]BeneficiaryResponseStruct, len(beneficiaries))
	for i, b := range beneficiaries {
		result[i] = BeneficiaryResponseStruct{
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
