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

func (o *OTPHandler) RequestOTP(c *gin.Context) {
	var req RequestOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	purpose, err := parsePurpose(req.Purpose)
	if err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	channel, err := parseChannel(req.Channel)
	if err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	if _, err := o.manager.Issue(c.Request.Context(), IssueOTPInput{
		Purpose:        purpose,
		Channel:        channel,
		VerificationID: req.VerificationID,
	}); err != nil {
		writeOTPError(c, err)
		return
	}

	c.JSON(200, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP sent successfully",
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

	purpose, err := parsePurpose(req.Purpose)
	if err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	channel, err := parseChannel(req.Channel)
	if err != nil {
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	result, err := o.manager.Verify(c.Request.Context(), VerifyOTPInput{
		Code:           req.OTP,
		VerificationID: req.VerificationID,
		Channel:        channel,
		Purpose:        purpose,
	})

	if err != nil {
		writeOTPError(c, err)
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
