package otp

import (
	"errors"
	"log"
	"neat_mobile_app_backend/internal/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type OTPHandler struct {
	manager OTPManager
}

func NewOTPHandler(manager OTPManager) *OTPHandler {
	return &OTPHandler{manager: manager}
}

var (
	errBadPurpose = errors.New("invalid purpose")
	errBadChannel = errors.New("invalid channel")
)

func (o *OTPHandler) RequestOTP(c *gin.Context) {
	var req RequestOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	purpose, err := parsePurpose(req.Purpose)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := parseChannel(req.Channel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "OTP sent successfully",
	})
}

func (o *OTPHandler) VerifyOTP(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	purpose, err := parsePurpose(req.Purpose)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := parseChannel(req.Channel)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, response.APIResponse[VerifyOTPResponse]{
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
		return "", errBadPurpose
	}
}

func parseChannel(v string) (Channel, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(ChannelSMS):
		return ChannelSMS, nil
	case string(ChannelEmail):
		return ChannelEmail, nil
	default:
		return "", errBadChannel
	}
}

func writeOTPError(c *gin.Context, err error) {
	log.Printf("otp error: %v", err)

	var status int
	var message string

	if strings.HasPrefix(err.Error(), "sms send failed with status:") {
		status = http.StatusBadGateway
		message = "SMS delivery failed"
	} else {
		switch err.Error() {
		case "too many requests":
			status = http.StatusTooManyRequests
			message = "Too many requests"
		case "invalid otp":
			status = http.StatusUnauthorized
			message = "Invalid OTP"
		case "invalid email", "invalid Nigerian number", "unsupported channel":
			status = http.StatusBadRequest
			message = err.Error()
		case "sms service not configured":
			status = http.StatusServiceUnavailable
			message = "SMS service not configured"
		default:
			status = http.StatusInternalServerError
			message = "Internal server error"
		}
	}

	c.AbortWithStatusJSON(status, response.APIResponse[any]{
		Status: "error",
		Error:  &response.APIError{Message: message},
	})
}
