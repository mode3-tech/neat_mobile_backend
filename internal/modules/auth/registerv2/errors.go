package registerv2

import (
	"errors"
	"fmt"

	"neat_mobile_app_backend/internal/modules/auth/otp"
)

// userError is an error whose message is written for the customer and is
// safe to return to the client. Any other error the response mapper doesn't
// recognise is internal (DB errors, provider plumbing, misconfiguration) and
// reaches the client only as a generic message - see Handler.writeError.
type userError struct {
	msg string
	err error // optional cause, kept for logs and errors.Is/As
}

func (e *userError) Error() string { return e.msg }
func (e *userError) Unwrap() error { return e.err }

// userErrorf builds a client-safe error.
func userErrorf(format string, args ...any) error {
	return &userError{msg: fmt.Sprintf(format, args...)}
}

// asUserError marks an existing error's message as client-safe (e.g. the
// password rules from authchecker). nil stays nil.
func asUserError(err error) error {
	if err == nil {
		return nil
	}
	return &userError{msg: err.Error(), err: err}
}

func isUserError(err error) bool {
	var ue *userError
	return errors.As(err, &ue)
}

// otpError maps the OTP manager's plain errors ("invalid otp", "too many
// requests", ...) to the app errors the response mapper renders; anything
// else stays internal.
func otpError(err error) error {
	if mapped := otp.MapOTPError(err); mapped != nil {
		return mapped
	}
	return err
}

// ErrAlatAccountGenerationPending means ALAT already has an account being
// generated for these details - typically a retry after an earlier tier 2
// request it accepted ("... provided already exist for this channel" with
// accountGenerationStatus "Pending"). The account will appear, so it's
// treated like an accepted creation, not a failure.
var ErrAlatAccountGenerationPending = errors.New("alat account generation is already pending for these details")
