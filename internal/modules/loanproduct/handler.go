package loanproduct

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

func (h *Handler) GetLoanProducts(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	loanProducts, err := h.service.GetAllLoanProducts(c.Request.Context())
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]PartialLoanProduct]{
		Status:  "success",
		Message: "Loan products fetched successfully",
		Data:    &loanProducts,
	})
}

func (h *Handler) ApplyForLoan(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))

	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var req LoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("loan handler: Error binding JSON: %v", err)
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	loanSummary, err := h.service.ApplyForLoan(c.Request.Context(), req, mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[*ApplyForLoanResponse]{
		Status:  "success",
		Message: "Loan application submitted successfully",
		Data:    &loanSummary,
	})
}

func (h *Handler) GetAllLoans(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetAllLoans(c.Request.Context(), mobileUserID)

	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]CoreCustomerLoanItem]{
		Status:  "success",
		Message: "Loans fetched successfully",
		Data:    &resp,
	})
}

func (h *Handler) GetLoanWithID(c *gin.Context) {
	loanID := strings.TrimSpace(c.Query("loan_id"))
	if loanID == "" {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
}

func (h *Handler) GetActiveLoans(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetActiveLoans(c.Request.Context(), mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]ActiveLoanItem]{
		Status:  "success",
		Message: "Active loans fetched successfully",
		Data:    &resp,
	})
}

func (h *Handler) GetLoanHistory(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetLoanHistory(c.Request.Context(), mobileUserID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]LoanHistoryItem]{
		Status:  "success",
		Message: "Loan history fetched successfully",
		Data:    &resp,
	})
}

func (h *Handler) GetLoanDetails(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	loanID := strings.TrimSpace(c.Param("loan_id"))
	if loanID == "" {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetLoanDetails(c.Request.Context(), mobileUserID, loanID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := &LoanDetails{
		TotalLoanAmount:    resp.Details.TotalLoanAmount,
		AmountRepaid:       resp.Details.AmountRepaid,
		OutstandingBalance: resp.Details.OutstandingBalance,
		DueDate:            resp.Details.DueDate,
		RepaymentHistory:   resp.Details.RepaymentHistory,
	}

	c.JSON(http.StatusOK, response.APIResponse[*LoanDetails]{
		Status:  "success",
		Message: "Loan details fetched successfully",
		Data:    &dto,
	})
}

func (h *Handler) GetLoanHistoryByLoanID(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	loanID := strings.TrimSpace(c.Param("loan_id"))
	if loanID == "" {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetLoanHistoryByLoanID(c.Request.Context(), mobileUserID, loanID)
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[[]LoanHistoryItem]{
		Status:  "success",
		Message: "Loan history fetched successfully",
		Data:    &resp,
	})
}

func (h *Handler) GetRepaymentSchedule(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	loanID := strings.TrimSpace(c.Query("loan_id"))

	if loanID == "" {
		mapped := response.MapError(appErr.ErrMissingRequiredQueryParameter)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp, err := h.service.GetLoanRepayments(c.Request.Context(), mobileUserID, loanID)

	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	dto := &LoanRepayment{
		LoanProductType:   resp.Repayment.LoanProductType,
		LoanAmount:        resp.Repayment.LoanAmount,
		TotalRepayment:    resp.Repayment.TotalRepayment,
		PeriodicRepayment: resp.Repayment.PeriodicRepayment,
		LoanDuration:      resp.Repayment.LoanDuration,
		AmountPaid:        resp.Repayment.AmountPaid,
		YetToPay:          resp.Repayment.YetToPay,
		InterestRate:      resp.Repayment.InterestRate,
	}

	c.JSON(http.StatusOK, response.APIResponse[*LoanRepayment]{
		Status:  "success",
		Message: "Loan repayments fetched successfully",
		Data:    &dto,
	})
}

func (h *Handler) HandleManualRepayment(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		mapped := response.MapError(appErr.ErrUnauthorized)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var req ManualRepaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	err := h.service.MakeManualRepayment(c.Request.Context(), mobileUserID, req)
	if err != nil {
		mapped := response.MapError(err)
		if strings.Contains(mapped.Error.Message, appErr.ErrIncorrectTransactionPin.Error()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    mapped.Error.Code,
					Message: err.Error(),
				},
			})
			return
		}
		if strings.Contains(mapped.Error.Message, ErrTooManyTransactionPinAttempts.Error()) {
			c.AbortWithStatusJSON(http.StatusForbidden, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    mapped.Error.Code,
					Message: err.Error(),
				},
			})
			return
		}
		if strings.Contains(mapped.Error.Message, ErrTransactionPinTemporarilyLocked.Error()) {
			c.AbortWithStatusJSON(http.StatusForbidden, response.APIResponse[any]{
				Status: "error",
				Error: &response.APIError{
					Code:    mapped.Error.Code,
					Message: err.Error(),
				},
			})
			return
		}
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Repayment successful",
	})
}
