package otp

import (
	"errors"
	"log"
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

	c.JSON(http.StatusOK, gin.H{"message": "otp sent"})
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
		Code:        req.OTP,
		Destination: req.Destination,
		Channel:     channel,
		Purpose:     purpose,
	})

	if err != nil {
		writeOTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, VerifyOTPResponse{
		Message:        "OTP verification was successful",
		VerificationID: result.VerificationID,
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

	if strings.HasPrefix(err.Error(), "sms send failed with status:") {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "sms delivery failed"})
		return
	}

	switch err.Error() {
	case "too many requests":
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
	case "invalid otp":
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid otp"})
	case "invalid email", "invalid Nigerian number", "unsupported channel":
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case "sms service not configured":
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "sms service not configured"})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
