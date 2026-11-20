package registerv2

import (
	"errors"
	"log"
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
	h.writeVerificationSuccess(c, verificationID, "", false, "Phone number verified.")
}

// RequestEmailOTP sends an OTP to the given email address, mirroring
// RequestPhoneOTP but over the email channel.
func (h *Handler) RequestEmailOTP(c *gin.Context) {
	var request OptimusEmailValidationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	otpID, err := h.service.RequestEmailOTP(c.Request.Context(), request.Email)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[RequestEmailOTPResponse]{
		Status:  "success",
		Message: "OTP sent. Confirm the code to complete email verification.",
		Data:    &RequestEmailOTPResponse{OTPID: otpID},
	})
}

// VerifyEmailOTP confirms the code sent by RequestEmailOTP and returns the
// verification id for use in the final /register call.
func (h *Handler) VerifyEmailOTP(c *gin.Context) {
	var request VerifyEmailOTPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, err := h.service.VerifyEmailOTP(c.Request.Context(), request.OTPID, request.Code)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeVerificationSuccess(c, verificationID, "", false, "Email verified.")
}

func (h *Handler) ValidateBVN(c *gin.Context) {
	var request ValidateBVNRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	verificationID, providerReferenceID, requiresOTP, err := h.service.ValidateBVN(c.Request.Context(), request.toOptimusRequest())
	if err != nil {
		h.writeError(c, err)
		return
	}

	message := "BVN validated successfully."
	switch {
	case requiresOTP:
		message = "BVN validated. Confirm the OTP sent to you to continue."
	case verificationID == "":
		// Providus: nothing was checked here; it validates the BVN when the
		// account is opened.
		message = "Your BVN will be verified when your account is opened. Continue to the next step."
	}
	h.writeVerificationSuccess(c, verificationID, providerReferenceID, requiresOTP, message)
}

// VerifyOTP confirms the OTP Optimus sent for a reference id (e.g. from
// ValidateBVN's provider_reference_id). Generic across
// whatever triggered the OTP challenge.
func (h *Handler) VerifyOTP(c *gin.Context) {
	var request VerifyOTPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	if err := h.service.VerifyOTP(c.Request.Context(), request.PhoneNo, request.OTPToken, request.Email, request.ReferenceID); err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP verified successfully.",
	})
}

// ResendOTP asks Optimus to resend the OTP tied to a reference id.
func (h *Handler) ResendOTP(c *gin.Context) {
	var request ResendOTPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	newReferenceID, err := h.service.ResendOTP(c.Request.Context(), request.ReferenceID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[ResendOTPResponse]{
		Status:  "success",
		Message: "OTP resent successfully.",
		Data:    &ResendOTPResponse{ReferenceID: newReferenceID},
	})
}

func (h *Handler) Register(c *gin.Context) {
	// Never log this body: it carries the password, transaction PIN, BVN, NIN
	// and face image.
	var request OptimusRegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("registerv2: register request binding failed: %v", err)
		h.writeInvalidRequest(c)
		return
	}

	resp, err := h.service.Register(c.Request.Context(), request, c.ClientIP())
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeRegistration(c, resp)
}

// RegistrationStatus lets a client follow a registration /register returned
// as pending, and collects its session once it completes.
func (h *Handler) RegistrationStatus(c *gin.Context) {
	var request RegistrationStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}
	resp, err := h.service.RegistrationStatus(c.Request.Context(), request.RegistrationID, request.ClaimToken, c.ClientIP())
	if err != nil {
		if errors.Is(err, errRegistrationNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, response.APIResponse[any]{
				Status: "error",
				Error:  &response.APIError{Code: "REGISTRATION_NOT_FOUND", Message: err.Error()},
			})
			return
		}
		if errors.Is(err, errRegistrationClaimed) {
			c.AbortWithStatusJSON(http.StatusConflict, response.APIResponse[any]{
				Status: "error",
				Error:  &response.APIError{Code: "REGISTRATION_ALREADY_CLAIMED", Message: err.Error()},
			})
			return
		}
		h.writeError(c, err)
		return
	}
	h.writeRegistration(c, resp)
}

// writeRegistration answers 200 for a completed registration and 202 while
// it's still pending (or failed/expired, with the reason in data.message).
func (h *Handler) writeRegistration(c *gin.Context, resp *RegisterResponse) {
	code, message := http.StatusOK, "Registration successful."
	if resp.LoginRequired {
		message = resp.Message
	}
	switch resp.Status {
	case RegistrationStatusPending:
		code, message = http.StatusAccepted, "Your account is being opened. Check the registration status shortly."
		if resp.Message != "" {
			message = resp.Message
		}
	case RegistrationStatusFailed, RegistrationStatusExpired:
		message = resp.Message
	}
	c.JSON(code, response.APIResponse[RegisterResponse]{
		Status:  "success",
		Message: message,
		Data:    resp,
	})
}

func (h *Handler) ValidateBVNWithFace(c *gin.Context) {
	var request BVNWithFaceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeInvalidRequest(c)
		return
	}

	faceCheckID, err := h.service.ValidateBVNWithFace(c.Request.Context(), request)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, response.APIResponse[BVNWithFaceResponse]{
		Status:  "success",
		Message: "BVN with face verification was successful.",
		Data: &BVNWithFaceResponse{
			BVNWithFaceVerificationID: faceCheckID,
		},
	})
}

func (h *Handler) GetAddressDropDownList(c *gin.Context) {
	data, err := h.service.GetAddressDropDownList(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Address dropdown options retrieved successfully.",
		Data:    &data,
	})
}

func (h *Handler) writeVerificationSuccess(c *gin.Context, verificationID, providerReferenceID string, requiresOTP bool, message string) {
	data := OptimusVerificationResponse{VerificationID: verificationID, ProviderReferenceID: providerReferenceID, RequiresOTP: requiresOTP}
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

// writeError renders registration errors without leaking internals:
//   - errors the response mapper knows keep their mapping (ALAT's own
//     validation messages included);
//   - an ALAT call that got no answer is "temporarily unavailable", not its
//     plumbing text;
//   - messages written for the customer (userError, e.g. Optimus's
//     ResponseMessage or "phone number is not verified") are returned as 422;
//   - anything else (DB errors, misconfiguration) is logged in full and
//     returned as a generic 500.
func (h *Handler) writeError(c *gin.Context, err error) {
	mapped := response.MapError(err)

	var alatErr *appErr.AlatError
	switch {
	case errors.As(err, &alatErr) && alatErr.NoResponse:
		log.Printf("registerv2: %s %s: wema unavailable: %v", c.Request.Method, c.FullPath(), err)
		mapped.Status = http.StatusServiceUnavailable
		mapped.Error.Code = "PROVIDER_UNAVAILABLE"
		mapped.Error.Message = "Wema is temporarily unavailable, please try again shortly."
	case mapped.Status != http.StatusInternalServerError:
		// Mapped by the shared error mapper.
	case isUserError(err):
		mapped.Status = http.StatusUnprocessableEntity
		mapped.Error.Code = "REGISTRATION_VALIDATION_FAILED"
		mapped.Error.Message = err.Error()
	default:
		log.Printf("registerv2: %s %s: internal error: %v", c.Request.Method, c.FullPath(), err)
		mapped.Error.Code = "INTERNAL_ERROR"
		mapped.Error.Message = "Something went wrong on our side. Please try again."
	}
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}
