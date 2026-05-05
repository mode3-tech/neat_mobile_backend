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

	case appErr.ErrRegistrationAlreadyInProgress:
		return ErrorMapping{
			Status: http.StatusConflict,
			Error: APIError{
				Code:    "REGISTRATION_IN_PROGRESS",
				Message: "registration already in progress",
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

	case appErr.ErrEmailPhoneMismatch:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "EMAIL_PHONE_MISMATCH",
				Message: "can not verify that email and phone belong to the same person",
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

	case appErr.ErrInvalidDateFrom, appErr.ErrInvalidDateTo, appErr.ErrInvalidDateRange:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_DATE",
				Message: "invalid date range",
			},
		}

	case appErr.ErrUnderaged:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "UNDERAGED",
				Message: "user is underaged",
			},
		}

	case appErr.ErrInvalidLoanAmount:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_AMOUNT",
				Message: "invalid loan amount",
			},
		}

	case appErr.ErrInvalidLoanProduct:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_PRODUCT",
				Message: "invalid loan product",
			},
		}

	case appErr.ErrIncompleteKYC:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INCOMPLETE_KYC",
				Message: "kyc is incomplete",
			},
		}

	case appErr.ErrIneligibleBusinessAge:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INELIGIBLE_BUSINESS_AGE",
				Message: "business has to be at least 1 year old to be eligible for a loan",
			},
		}

	case appErr.ErrIneligibleForLoan:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INELIGIBLE_FOR_LOAN",
				Message: "user has active loan or has defaulted on a previous loan",
			},
		}

	case appErr.ErrInvalidBusinessValue:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_BUSINESS_VALUE",
				Message: "business value is too low to be eligible for a loan",
			},
		}

	case appErr.ErrInvalidLoanTerm:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_TERM",
				Message: "invalid loan term",
			},
		}

	case appErr.ErrInvalidSavingsAmount:
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_SAVINGS_AMOUNT",
				Message: "savings amount must be at least 50 NGN",
			},
		}

	case appErr.ErrInvalidVerificationID:
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_VERIFICATION_ID",
				Message: "invalid verification id",
			},
		}

	case appErr.ErrInvalidChannel:
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "INVALID_CHANNEL",
				Message: "invalid channel",
			},
		}

	case appErr.ErrTooManyRequests:
		return ErrorMapping{
			Status: http.StatusTooManyRequests,
			Error: APIError{
				Code:    "TOO_MANY_REQUESTS",
				Message: "too many requests, please try again later",
			},
		}

	case appErr.ErrInvalidEmail:
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_EMAIL",
				Message: "invalid email",
			},
		}

	case appErr.ErrUnableToGenerateOTP:
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_GENERATE_OTP",
				Message: "OTP generation failed, please try again",
			},
		}

	case appErr.ErrUnableToHashOTP:
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_HASH_OTP",
				Message: "error occured while sending OTP, please try again",
			},
		}

	case appErr.ErrInvalidFileFormat:
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_FILE_FORMAT",
				Message: "use a valid file format",
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
