package response

import (
	appErr "neat_mobile_app_backend/internal/errors"
	"net/http"
)

type ErrorMapping struct {
	Status int
	Error  APIError
}

func MapError(err error) ErrorMapping {
	switch err {
	case appErr.ErrInvalidCredentials:
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "AUTH_INVALID_CREDENTIALS",
				Message: "invalid credentials",
			},
		}

	case appErr.ErrUnauthorized:
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "UNAUTHORIZED",
				Message: "unauthorized access",
			},
		}

	case appErr.ErrUserExists:
		return ErrorMapping{
			Status: http.StatusConflict,
			Error: APIError{
				Code:    "USER_EXISTS",
				Message: "user already exists",
			},
		}

	case appErr.ErrNotFound:
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NOT_FOUND",
				Message: "resource not found",
			},
		}

	case appErr.ErrBVNNotFound:
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "BVN_NOT_FOUND",
				Message: "bvn verification not found",
			},
		}

	case appErr.ErrNINNotFound:
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NIN_NOT_FOUND",
				Message: "nin verification not found",
			},
		}

	case appErr.ErrInvalidBVN:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_BVN",
				Message: "invalid bvn",
			},
		}

	case appErr.ErrInvalidNIN:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_NIN",
				Message: "invalid nin",
			},
		}

	case appErr.ErrPhoneNotFound:
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "PHONE_NOT_FOUND",
				Message: "phone verification not found",
			},
		}

	case appErr.ErrPhoneMismatch:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "PHONE_MISMATCH",
				Message: "verified phone and phone number do not match",
			},
		}

	case appErr.ErrEmailNotFound:
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "EMAIL_NOT_FOUND",
				Message: "email verification not found",
			},
		}

	case appErr.ErrNINAndBVNMismatch:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "NIN_BVN_MISMATCH",
				Message: "could not verify bvn and nin belongs to the same person due to a mismatch in names of date of births",
			},
		}

	case appErr.ErrPasswordMismatch:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "PASSWORD_MISMATCH",
				Message: "password and confirm password do not match",
			},
		}

	case appErr.ErrTransactionPinMismatch:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "TRANSACTION_PIN_MISMATCH",
				Message: "transaction pin and confirm transaction pin do not match",
			},
		}

	case appErr.ErrInvalidSession:
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INVALID_SESSION",
				Message: "invalid or expired session",
			},
		}

	case appErr.ErrInvalidOTP:
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INVALID_OTP",
				Message: "invalid otp",
			},
		}

	case appErr.ErrInvalidPhone:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_PHONE",
				Message: "invalid phone number",
			},
		}

	default:
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "something went wrong",
			},
		}
	}
}
