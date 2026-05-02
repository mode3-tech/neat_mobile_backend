package errors

import "errors"

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserExists             = errors.New("user already exists")
	ErrNotFound               = errors.New("not found")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrBVNNotFound            = errors.New("bvn not found")
	ErrNINNotFound            = errors.New("nin not found")
	ErrInvalidNIN             = errors.New("invalid nin")
	ErrInvalidBVN             = errors.New("invalid bvn")
	ErrPhoneNotFound          = errors.New("phone not found")
	ErrPhoneMismatch          = errors.New("phones do not match")
	ErrEmailNotFound          = errors.New("email not found")
	ErrNINAndBVNMismatch      = errors.New("nin and bvn do not match")
	ErrPasswordMismatch       = errors.New("passwords do not match")
	ErrTransactionPinMismatch = errors.New("transaction pins do not match")
	ErrMissingDeviceID        = errors.New("missing device id")
	ErrMissingUserID          = errors.New("missing user id")
	ErrDeviceNotAllowed       = errors.New("device not allowed")
	ErrInvalidSession         = errors.New("invalid session")
	ErrMissingOTP             = errors.New("otp is required")
	ErrInvalidOTP             = errors.New("invalid otp")
	ErrMissingPublicKey       = errors.New("public key is required")
)
