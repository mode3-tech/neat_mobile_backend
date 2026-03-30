package otp

import (
	"context"
	"time"
)

type OTPManager interface {
	Issue(ctx context.Context, in IssueOTPInput) (*IssueOTPResult, error)
	Verify(ctx context.Context, in VerifyOTPInput) (*VerifyOTPResult, error)
}

type IssueOTPInput struct {
	Purpose     Purpose
	Channel     Channel
	Destination string
	UserID      string
	TTL         time.Duration
	MaxAttempts int
	MaxResends  int
}

type IssueOTPResult struct {
	OTPID      string
	ExpiresAt  time.Time
	NextSendAt *time.Time
}

type VerifyOTPInput struct {
	Purpose     Purpose
	OTPID       string  // if set, look up by ID (new-device flow)
	Channel     Channel // required when OTPID is empty
	Destination string
	Code        string // used when OTPID is empty
}

type VerifyOTPResult struct {
	OTPID          string
	UserID         string
	VerifiedAt     time.Time
	VerificationID string // ID of the verification record that was created
}
