package loanproduct

import (
	"errors"
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

func (h *Handler) GetLoanProducts(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))

	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	loanProducts, err := h.service.GetAllLoanProducts(c.Request.Context())

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "loan products fetch was successful", "products": loanProducts})
}

func (h *Handler) ApplyForLoan(c *gin.Context) {
	var req LoanRequest
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))

	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if isBadRequestApplyForLoanError(err) {
			if strings.Contains(err.Error(), "LoanRequest.BusinessStartDate") {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "business_start_date is missing or in the wrong format"})
				return
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid req body"})
		return
	}
	loanSummary, err := h.service.ApplyForLoan(c.Request.Context(), req, userID)
	if err != nil {
		_ = c.Error(err)
		abortApplyForLoanError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": loanSummary.Message, "application_ref": loanSummary.ApplicationRef, "loan_status": loanSummary.LoanStatus, "summary": loanSummary.Summary})
}

func abortApplyForLoanError(c *gin.Context, err error) {
	if isBadRequestApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": normalizeApplyForLoanErrorMessage(err)})
		return
	}

	if isUnprocessableEntityApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	if isConflictApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if isNotFoundApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if isTooManyRequestsApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	if isUnauthorizedApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	if isBadGatewayApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	if isServiceUnavailableApplyForLoanError(err) {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
}

func normalizeApplyForLoanErrorMessage(err error) string {
	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "LoanRequest.BusinessStartDate") {
		return "business_start_date is missing or in the wrong format"
	}

	return msg
}

func isBadRequestApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "dob is required":
		return true
	case "unable to get age from dob, check dob again":
		return true
	case "bvn is required":
		return true
	case "invalid bvn number":
		return true
	}

	if strings.HasPrefix(msg, "invalid dob format") {
		return true
	}

	if strings.Contains(msg, "LoanRequest.BusinessStartDate") {
		return true
	}

	return false
}

func isUnprocessableEntityApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "business value should be digits":
		return true
	case "loan amount must be digits":
		return true
	case "invalid loan amount":
		return true
	case "user is below the legal age to borrow a loan":
		return true
	case "loan product type is invalid":
		return true
	case "loan amount must be in the range of the min and max amount of selected loan product":
		return true
	case "invalid business value":
		return true
	case "loan amount must be less than or equal to total business value":
		return true
	case "business must be at least a year old":
		return true
	case "user's bvn is not verified":
		return true
	case "user's nin is not verified":
		return true
	case "user's phone is not verified":
		return true
	case "customer does not exist on core app":
		return true
	case "customer has reached the maximum number of active loans":
		return true
	case "customer has an outstanding defaulted loan":
		return true
	case "loan term must be greater than zero":
		return true
	case "invalid format, expected MM/YYYY":
		return true
	}

	return false
}

func isConflictApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "multiple core customers matched this bvn":
		return true
	}

	return false
}

func isNotFoundApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "current user does not exist":
		return true
	case "loan rule not found":
		return true
	}

	return false
}

func isUnauthorizedApplyForLoanError(err error) bool {
	if errors.Is(err, ErrIncorrectTransactionPin) {
		return true
	}

	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "unable to get age from dob, check dob again":
		return true
	}

	return false
}

func isTooManyRequestsApplyForLoanError(err error) bool {
	return errors.Is(err, ErrTooManyTransactionPinAttempts) || errors.Is(err, ErrTransactionPinTemporarilyLocked)
}

func isBadGatewayApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "core app returned empty customer match response":
		return true
	case "core app returned empty matched customer":
		return true
	case "an error occured while looking up customer on the core app":
		return true
	case "customer id is required":
		return true
	case "invalid customer id":
		return true
	case "loan id is required":
		return true
	case "invalid loan id":
		return true
	}

	return strings.HasPrefix(msg, "cba match customer by bvn failed with status ") ||
		strings.HasPrefix(msg, "cba get customer loans failed with status ") ||
		strings.HasPrefix(msg, "cba get loan detail failed with status ")
}

func isServiceUnavailableApplyForLoanError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "core customer finder is not configured":
		return true
	case "core loan finder is not configured":
		return true
	case "cba base url is not configured":
		return true
	case "cba internal key is not configured":
		return true
	}

	return false
}

func (h *Handler) GetAllLoans(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))

	if userID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "user id missing"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	resp, err := h.service.GetAllLoans(c.Request.Context(), userID, deviceID)

	if err != nil {
		_ = c.Error(err)

		if isBadRequestGetAllLoansError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			return
		}

		if isNotFoundGetAllLoansError(err) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user or user's loan not found"})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isBadRequestGetAllLoansError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "invalid user id":
		return true
	default:
		return false
	}
}

func isNotFoundGetAllLoansError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "no user found":
		return true
	case "user has not existing loan":
		return true
	default:
		return false
	}
}

func (h *Handler) GetLoanWithID(c *gin.Context) {
	loanID := strings.TrimSpace(c.Query("loan_id"))

	if loanID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid query parameter"})
		return
	}
}

func (h *Handler) GetActiveLoans(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	resp, err := h.service.GetActiveLoans(c.Request.Context(), userID, deviceID)
	if err != nil {
		_ = c.Error(err)

		msg := strings.TrimSpace(err.Error())
		switch msg {
		case "invalid user id":
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		case "no user found", "user has not active loans yet":
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": msg})
		case "device not found", "device not allowed":
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetLoanHistory(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	resp, err := h.service.GetLoanHistory(c.Request.Context(), userID, deviceID)
	if err != nil {
		_ = c.Error(err)
		msg := strings.TrimSpace(err.Error())
		switch msg {
		case "no user found", "no loan history found":
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": msg})
		case "device not found", "device not allowed":
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetLoanDetails(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	loanID := strings.TrimSpace(c.Param("loan_id"))
	if loanID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "loan_id is required"})
		return
	}

	resp, err := h.service.GetLoanDetails(c.Request.Context(), userID, deviceID, loanID)
	if err != nil {
		_ = c.Error(err)
		msg := strings.TrimSpace(err.Error())
		switch msg {
		case "invalid loan id":
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
		case "device not found", "device not allowed":
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetLoanHistoryByLoanID(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	loanID := strings.TrimSpace(c.Param("loan_id"))
	if loanID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "loan_id is required"})
		return
	}

	resp, err := h.service.GetLoanHistoryByLoanID(c.Request.Context(), userID, deviceID, loanID)
	if err != nil {
		_ = c.Error(err)
		msg := strings.TrimSpace(err.Error())
		switch msg {
		case "invalid loan id":
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
		case "device not found", "device not allowed":
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, try again"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetRepaymentSchedule(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Device-ID header"})
		return
	}

	loanID := strings.TrimSpace(c.Query("loan_id"))

	if loanID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "loan id is missing in the query"})
		return
	}

	resp, err := h.service.GetLoanRepayments(c.Request.Context(), userID, deviceID, loanID)

	if err != nil {
		if isBadRequestGetRepaymentScheduleError(err) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "an error occured"})
		return
	}

	c.JSON(http.StatusOK, resp)

}

func isBadRequestGetRepaymentScheduleError(err error) bool {
	msg := strings.TrimSpace(err.Error())

	switch msg {
	case "invalid loan id":
		return true
	default:
		return false
	}
}

func (h *Handler) HandleManualRepayment(c *gin.Context) {
	mobileUserID := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if mobileUserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ManualRepaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	resp, err := h.service.MakeManualRepayment(c.Request.Context(), mobileUserID, deviceID, req)
	if err != nil {
		handleHandleManualRepaymentError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleHandleManualRepaymentError(c *gin.Context, err error) {
	if errors.Is(err, ErrTooManyTransactionPinAttempts) || errors.Is(err, ErrTransactionPinTemporarilyLocked) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	msg := strings.TrimSpace(err.Error())

	if strings.Contains(msg, "wrong transaction pin: you have") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
		return
	}

	switch msg {
	case "mobile user id is required", "device id is required", "device not found", "device not allowed",
		"wrong transaction pin":
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	case "transaction pin locked":
		c.AbortWithStatusJSON(http.StatusLocked, gin.H{"error": msg})
	case "insufficient balance":
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
	case "repayment service is not configured", "wallet service is not configured":
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": msg})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	}
}
