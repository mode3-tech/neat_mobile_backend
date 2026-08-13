package registerv2

import (
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RequestPhoneOTP(c *gin.Context) {
	var request RequestPhoneOTPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	otpID, err := h.service.RequestPhoneOTP(c.Request.Context(), request.PhoneNumber)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[RequestPhoneOTPResponse]{
		Status:  "success",
		Message: "OTP sent. Confirm the code to complete phone verification.",
		Data:    &RequestPhoneOTPResponse{OTPID: otpID},
	})
}

func (h *Handler) VerifyPhoneOTP(c *gin.Context) {
	var request VerifyPhoneOTPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, err := h.service.VerifyPhoneOTP(c.Request.Context(), request.OTPID, request.Code)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeVerificationSuccess(c, verificationID, "Phone number verified.")
}

func (h *Handler) StartEmailVerification(c *gin.Context) {
	var request OptimusEmailValidationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, err := h.service.StartEmailVerification(c.Request.Context(), request.Email)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeVerificationSuccess(c, verificationID, "Email verification started. Confirm the email OTP to complete verification.")
}

func (h *Handler) ValidateBVN(c *gin.Context) {
	var request OptimusBVNValidationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, err := h.service.ValidateBVN(c.Request.Context(), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeVerificationSuccess(c, verificationID, "BVN validated successfully.")
}

func (h *Handler) ValidateNIN(c *gin.Context) {
	var request OptimusNINValidationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, err := h.service.ValidateNIN(c.Request.Context(), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeVerificationSuccess(c, verificationID, "NIN validated successfully.")
}

func (h *Handler) Register(c *gin.Context) {
	var request OptimusRegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	resp, err := h.service.Register(c.Request.Context(), request, c.ClientIP())
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[RegisterResponse]{
		Status:  "success",
		Message: "Registration successful.",
		Data:    resp,
	})
}

func (h *Handler) writeVerificationSuccess(c *gin.Context, verificationID, message string) {
	data := OptimusVerificationResponse{VerificationID: verificationID}
	c.JSON(http.StatusOK, response.APIResponse[OptimusVerificationResponse]{
		Status:  "success",
		Message: message,
		Data:    &data,
	})
}

func (h *Handler) writeInvalidRequest(c *gin.Context) {
	mapped := response.MapError(appErr.ErrInvalidRequestBody)
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	// Optimus supplies a user-facing ResponseMessage. Preserve that message for
	// this public flow instead of replacing it with the generic error mapper text.
	mapped := response.MapError(err)
	if mapped.Status == http.StatusInternalServerError {
		mapped.Status = http.StatusUnprocessableEntity
		mapped.Error.Code = "OPTIMUS_VALIDATION_FAILED"
		mapped.Error.Message = err.Error()
	}
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}
