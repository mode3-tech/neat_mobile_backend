package otp

import (
	"log"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
	"strings"

	"github.com/gin-gonic/gin"
)

type OTPHandler struct {
	manager OTPManager
}

func NewOTPHandler(manager OTPManager) *OTPHandler {
	return &OTPHandler{manager: manager}
}

func (o *OTPHandler) RequestSMSOTP(c *gin.Context) {
	var req RequestSMSOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	result, err := o.manager.Issue(c.Request.Context(), IssueOTPInput{
		Purpose:        signupOTPPurpose(req.Purpose),
		Channel:        ChannelSMS,
		VerificationID: req.VerificationID,
		Destination:    strings.TrimSpace(req.Destination),
	})
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp := RequestOTPResponse{
		OTPID: result.OTPID,
	}

	c.JSON(200, response.APIResponse[RequestOTPResponse]{
		Status:  "success",
		Message: "OTP sent successfully",
		Data:    &resp,
	})
}

func (o *OTPHandler) RequestEmailOTP(c *gin.Context) {
	var req RequestEmailOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var destination string

	if req.Destination == "" {
		destination = ""
	} else {
		destination = strings.TrimSpace(req.Destination)
	}
	result, err := o.manager.Issue(c.Request.Context(), IssueOTPInput{
		Purpose:        signupOTPPurpose(req.Purpose),
		Channel:        ChannelEmail,
		VerificationID: req.VerificationID,
		Destination:    destination,
	})
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp := RequestOTPResponse{
		OTPID: result.OTPID,
	}

	c.JSON(200, response.APIResponse[RequestOTPResponse]{
		Status:  "success",
		Message: "OTP sent successfully",
		Data:    &resp,
	})
}

func (o *OTPHandler) VerifyOTP(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
	result, err := o.manager.Verify(c.Request.Context(), VerifyOTPInput{
		Code:    req.OTP,
		OTPID:   req.OTPID,
		Purpose: signupOTPPurpose(req.Purpose),
	})

	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	resp := VerifyOTPResponse{
		VerificationID: result.VerificationID,
	}

	c.JSON(200, response.APIResponse[VerifyOTPResponse]{
		Status:  "success",
		Message: "OTP verified successfully",
		Data:    &resp,
	})
}

// signupOTPPurpose maps the optional request purpose to the internal Purpose
// for the signup OTP endpoints. It only distinguishes the primary signup
// contact (default) from a submitted alternate contact; the binding tag
// `oneof=signup submitted_contact` guarantees no other value reaches here.
func signupOTPPurpose(v string) Purpose {
	if strings.EqualFold(strings.TrimSpace(v), string(PurposeSubmittedContact)) {
		return PurposeSubmittedContact
	}
	return PurposeSignup
}

func parsePurpose(v string) (Purpose, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(PurposeLogin):
		return PurposeLogin, nil
	case string(PurposeSignup):
		return PurposeSignup, nil
	case string(PurposePasswordReset):
		return PurposePasswordReset, nil
	default:
		return "", appErr.ErrInvalidRequestBody
	}
}

func parseChannel(v string) (Channel, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(ChannelSMS):
		return ChannelSMS, nil
	case string(ChannelEmail):
		return ChannelEmail, nil
	default:
		return "", appErr.ErrInvalidRequestBody
	}
}

func writeOTPError(c *gin.Context, err error) {
	log.Printf("otp error: %v", err)

	var mappedErr error
	switch {
	case strings.HasPrefix(err.Error(), "sms send failed with status:"):
		mappedErr = appErr.ErrSMSDeliveryFailed
	default:
		switch err.Error() {
		case "too many requests":
			mappedErr = appErr.ErrTooManyRequests
		case "invalid otp":
			mappedErr = appErr.ErrInvalidOTP
		case "invalid email":
			mappedErr = appErr.ErrInvalidEmail
		case "invalid Nigerian number":
			mappedErr = appErr.ErrInvalidPhone
		case "unsupported channel":
			mappedErr = appErr.ErrInvalidChannel
		case "sms service not configured":
			mappedErr = appErr.ErrSMSServiceNotConfigured
		default:
			mappedErr = nil
		}
	}

	mapped := response.MapError(mappedErr)
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}
