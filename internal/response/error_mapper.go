package response

import (
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"net/http"
)

type ErrorMapping struct {
	Status int
	Error  APIError
}

func MapError(err error) ErrorMapping {
	switch {
	case errors.Is(err, appErr.ErrInvalidCredentials):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "AUTH_INVALID_CREDENTIALS",
				Message: "invalid credentials",
			},
		}

	case errors.Is(err, appErr.ErrUnauthorized):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "UNAUTHORIZED",
				Message: "unauthorized access",
			},
		}

	case errors.Is(err, appErr.ErrInvalidPassword):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_PASSWORD",
				Message: "password must contain at least one uppercase letter, one lowercase letter, one number, and one special character",
			},
		}

	case errors.Is(err, appErr.ErrBadRequest):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "BAD_REQUEST",
				Message: "bad request",
			},
		}

	case errors.Is(err, appErr.ErrMissingEmail):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_EMAIL",
				Message: appErr.ErrMissingEmail.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingBVNPhoneNumber):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_BVN_PHONE_NUMBER",
				Message: appErr.ErrMissingBVNPhoneNumber.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingBVNPhoneNumberAndEmail):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_BVN_PHONE_NUMBER_AND_EMAIL",
				Message: appErr.ErrMissingBVNPhoneNumberAndEmail.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingPhone):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_PHONE",
				Message: appErr.ErrMissingPhone.Error(),
			},
		}

	case errors.Is(err, appErr.ErrUserExists):
		return ErrorMapping{
			Status: http.StatusConflict,
			Error: APIError{
				Code:    "USER_EXISTS",
				Message: "user already exists",
			},
		}

	case errors.Is(err, appErr.ErrRegistrationAlreadyInProgress):
		return ErrorMapping{
			Status: http.StatusConflict,
			Error: APIError{
				Code:    "REGISTRATION_IN_PROGRESS",
				Message: "registration already in progress",
			},
		}

	case errors.Is(err, appErr.ErrInvalidChannel):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_CHANNEL",
				Message: "invalid channel",
			},
		}

	case errors.Is(err, appErr.ErrNotFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NOT_FOUND",
				Message: "resource not found",
			},
		}

	case errors.Is(err, appErr.ErrInvalidProductAmount):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_PRODUCT_AMOUNT",
				Message: "product amount mismatch",
			},
		}

	case errors.Is(err, appErr.ErrBVNNotFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "BVN_NOT_FOUND",
				Message: "bvn verification not found",
			},
		}

	case errors.Is(err, appErr.ErrNINNotFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NIN_NOT_FOUND",
				Message: "nin verification not found",
			},
		}

	case errors.Is(err, appErr.ErrInvalidBVN):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_BVN",
				Message: "invalid bvn",
			},
		}

	case errors.Is(err, appErr.ErrInvalidNIN):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_NIN",
				Message: "invalid nin",
			},
		}

	case errors.Is(err, appErr.ErrPhoneOrEmailNotFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "PHONE_OR_EMAIL_NOT_FOUND",
				Message: "kindly verify your phone or email",
			},
		}

	case errors.Is(err, appErr.ErrPhoneMismatch):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "PHONE_MISMATCH",
				Message: "verified phone and phone number do not match",
			},
		}

	case errors.Is(err, appErr.ErrEmailNotFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "EMAIL_NOT_FOUND",
				Message: "email verification not found",
			},
		}

	case errors.Is(err, appErr.ErrEmailPhoneMismatch):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "EMAIL_PHONE_MISMATCH",
				Message: "can not verify that email and phone belong to the same person",
			},
		}

	case errors.Is(err, appErr.ErrNINAndBVNMismatch):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "NIN_BVN_MISMATCH",
				Message: "could not verify bvn and nin belongs to the same person due to a mismatch in names or date of births",
			},
		}

	case errors.Is(err, appErr.ErrPasswordMismatch):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "PASSWORD_MISMATCH",
				Message: "password and confirm password do not match",
			},
		}

	case errors.Is(err, appErr.ErrTransactionPinMismatch):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "TRANSACTION_PIN_MISMATCH",
				Message: "transaction pin and confirm transaction pin do not match",
			},
		}

	case errors.Is(err, appErr.ErrInvalidSession):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INVALID_SESSION",
				Message: "invalid or expired session",
			},
		}

	case errors.Is(err, appErr.ErrInvalidOTP):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INVALID_OTP",
				Message: "invalid otp",
			},
		}

	case errors.Is(err, appErr.ErrInvalidPhone):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_PHONE",
				Message: "invalid phone number",
			},
		}

	case errors.Is(err, appErr.ErrInvalidDateFrom) || errors.Is(err, appErr.ErrInvalidDateTo) || errors.Is(err, appErr.ErrInvalidDateRange):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_DATE",
				Message: "invalid date range",
			},
		}

	case errors.Is(err, appErr.ErrUnderaged):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "UNDERAGED",
				Message: "user is underaged",
			},
		}

	case errors.Is(err, appErr.ErrInvalidLoanAmount):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_AMOUNT",
				Message: "invalid loan amount",
			},
		}

	case errors.Is(err, appErr.ErrInvalidLoanProduct):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_PRODUCT",
				Message: "invalid loan product",
			},
		}

	case errors.Is(err, appErr.ErrIncompleteKYC):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INCOMPLETE_KYC",
				Message: "kyc is incomplete",
			},
		}

	case errors.Is(err, appErr.ErrIneligibleBusinessAge):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INELIGIBLE_BUSINESS_AGE",
				Message: "business has to be at least 1 year old to be eligible for a loan",
			},
		}

	case errors.Is(err, appErr.ErrIneligibleForLoan):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INELIGIBLE_FOR_LOAN",
				Message: "you have active loan or has defaulted on a previous loan",
			},
		}

	case errors.Is(err, appErr.ErrUnprocessedAppliedLoanExists):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "UNPROCESSED_APPLIED_LOAN_EXISTS",
				Message: "you have applied loans that have not been processed yet",
			},
		}

	case errors.Is(err, appErr.ErrInvalidBusinessValue):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_BUSINESS_VALUE",
				Message: "business value is too low to be eligible for a loan",
			},
		}

	case errors.Is(err, appErr.ErrInvalidLoanTerm):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_LOAN_TERM",
				Message: "invalid loan term",
			},
		}

	case errors.Is(err, appErr.ErrInvalidSavingsAmount):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_SAVINGS_AMOUNT",
				Message: "savings amount must be at least 50 NGN",
			},
		}

	case errors.Is(err, appErr.ErrInvalidVerificationID):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_VERIFICATION_ID",
				Message: "We couldn't verify one of your details. Please restart the verification and try again.",
			},
		}

	case errors.Is(err, appErr.ErrInvalidChannel):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "INVALID_CHANNEL",
				Message: "invalid channel",
			},
		}

	case errors.Is(err, appErr.ErrTooManyRequests):
		return ErrorMapping{
			Status: http.StatusTooManyRequests,
			Error: APIError{
				Code:    "TOO_MANY_REQUESTS",
				Message: "too many requests, please try again later",
			},
		}

	case errors.Is(err, appErr.ErrInvalidEmail):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_EMAIL",
				Message: "invalid email",
			},
		}

	case errors.Is(err, appErr.ErrUnableToGenerateOTP):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_GENERATE_OTP",
				Message: "OTP generation failed, please try again",
			},
		}

	case errors.Is(err, appErr.ErrUnableToHashOTP):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_HASH_OTP",
				Message: "error occured while sending OTP, please try again",
			},
		}

	case errors.Is(err, appErr.ErrInvalidFileFormat):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_FILE_FORMAT",
				Message: "use a valid file format",
			},
		}

	case errors.Is(err, appErr.ErrFetchingAccountSummary):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ACCOUNT_SUMMARY_INTERNAL_ERROR",
				Message: appErr.ErrFetchingAccountSummary.Error(),
			},
		}

	case errors.Is(err, appErr.ErrUpdatingProfile):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "PROFILE_UPDATE_INTERNAL_ERROR",
				Message: appErr.ErrUpdatingProfile.Error(),
			},
		}

	case errors.Is(err, appErr.ErrGeneratingAccountStatement):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ACCOUNT_STATEMENT_GENERATION_INTERNAL_ERROR",
				Message: appErr.ErrGeneratingAccountStatement.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingAccountStatement):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ACCOUNT_STATEMENT_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingAccountStatement.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingLastestAccountStatement):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "LATEST_ACCOUNT_STATEMENT_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingLastestAccountStatement.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingTransactions):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "TRANSACTIONS_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingTransactions.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidRequestBody):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_REQUEST_BODY",
				Message: appErr.ErrInvalidRequestBody.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidQueryParameter):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_QUERY_PARAMETER",
				Message: appErr.ErrInvalidQueryParameter.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingRequiredQueryParameter):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_REQUIRED_QUERY_PARAMETER",
				Message: appErr.ErrMissingRequiredQueryParameter.Error(),
			},
		}

	case errors.Is(err, appErr.ErrNoTransactionsFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NO_TRANSACTIONS_FOUND",
				Message: appErr.ErrNoTransactionsFound.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidCursor):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_CURSOR",
				Message: "Invalid request body",
			},
		}

	case errors.Is(err, appErr.ErrDeviceNotAllowed):
		return ErrorMapping{
			Status: http.StatusForbidden,
			Error: APIError{
				Code:    "DEVICE_NOT_ALLOWED",
				Message: "Device is inactive or not trusted, please contact support to resolve this issue",
			},
		}

	case errors.Is(err, appErr.ErrUnrecognizedDevice):
		return ErrorMapping{
			Status: http.StatusForbidden,
			Error: APIError{
				Code:    "UNRECOGNIZED_DEVICE",
				Message: "We could not recognize the device you are using, please contact support to resolve this issue",
			},
		}

	case errors.Is(err, appErr.ErrNoLoansFound):
		return ErrorMapping{
			Status: http.StatusNotFound,
			Error: APIError{
				Code:    "NO_LOANS_FOUND",
				Message: "No loans found for user",
			},
		}

	case errors.Is(err, appErr.ErrIncorrectTransactionPin):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INCORRECT_TRANSACTION_PIN",
				Message: err.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidDOB):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_DOB",
				Message: appErr.ErrInvalidDOB.Error(),
			},
		}

	case errors.Is(err, appErr.ErrApplyingForLoan):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "LOAN_APPLICATION_INTERNAL_ERROR",
				Message: appErr.ErrApplyingForLoan.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingActiveLoans):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ACTIVE_LOANS_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingActiveLoans.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingLoanHistory):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "LOAN_HISTORY_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingLoanHistory.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingLoanDetails):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "LOAN_DETAILS_FETCHING_INTERNAL_ERROR",
				Message: appErr.ErrFetchingLoanDetails.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingDeviceID) || errors.Is(err, appErr.ErrMissingUserID):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_REQUIRED_FIELD",
				Message: err.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidSession):
		return ErrorMapping{
			Status: http.StatusUnauthorized,
			Error: APIError{
				Code:    "INVALID_SESSION",
				Message: "invalid or expired session",
			},
		}

	case errors.Is(err, appErr.ErrInvalidVerificationType):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_VERIFICATION_TYPE",
				Message: "cannot verify your account. kindly restart the verification process",
			},
		}

	case errors.Is(err, appErr.ErrMissingOTP):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_OTP",
				Message: "otp is required",
			},
		}

	case errors.Is(err, appErr.ErrUnableToGenerateResetToken):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_GENERATE_RESET_TOKEN",
				Message: "unable to generate password reset token, please try again",
			},
		}

	case errors.Is(err, appErr.ErrInvalidRequestBody):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_REQUEST_BODY",
				Message: "invalid request body",
			},
		}

	case errors.Is(err, appErr.ErrTransactionPinLocked):
		return ErrorMapping{
			Status: http.StatusForbidden,
			Error: APIError{
				Code:    "TRANSACTION_PIN_LOCKED",
				Message: appErr.ErrTransactionPinLocked.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMakingRepayment):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "REPAYMENT_INTERNAL_ERROR",
				Message: appErr.ErrMakingRepayment.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidNotificationPlatform):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "INVALID_NOTIFICATION_PLATFORM",
				Message: appErr.ErrInvalidNotificationPlatform.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidExpoToken):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "INVALID_EXPO_TOKEN",
				Message: "An unexpected error occured, please try again later",
			},
		}

	case errors.Is(err, appErr.ErrSendingPushNotification):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ERROR_SENDING_PUSH_NOTIFICATION",
				Message: appErr.ErrSendingPushNotification.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingNotifications):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ERROR_FETCHING_NOTIFICATIONS",
				Message: appErr.ErrFetchingNotifications.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingUnreadNotifications):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FETCHING_UNREAD_NOTIFICATIONS_FAILED",
				Message: appErr.ErrFetchingUnreadNotifications.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMarkingNotifications):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_MARK_NOTIFICATIONS",
				Message: appErr.ErrMarkingNotifications.Error(),
			},
		}

	case errors.Is(err, appErr.ErrDeletingPushToken):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ERROR_DELETING_EXPO_TOKEN",
				Message: appErr.ErrDeletingPushToken.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingRequiredPathParameter):
		return ErrorMapping{
			Status: http.StatusBadRequest,
			Error: APIError{
				Code:    "MISSING_REQUIRED_PATH_PARAMETER",
				Message: appErr.ErrMissingRequiredPathParameter.Error(),
			},
		}

	case errors.Is(err, appErr.ErrTogglingPushNotification):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "UNABLE_TO_TOGGLE_PUSH",
				Message: appErr.ErrTogglingPushNotification.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingBanks):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FETCHING_BANKS_ERROR",
				Message: appErr.ErrFetchingBanks.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingBankDetails):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FETCHING_BANK_DETAILS_ERROR",
				Message: appErr.ErrFetchingBankDetails.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidTransferAmount):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_TRANSFER_AMOUNT",
				Message: appErr.ErrInvalidTransferAmount.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFundsTransfer):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FAILED_TRANSFER",
				Message: appErr.ErrFundsTransfer.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMissingUserWallet):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "USER WALLET NOT FOUND",
				Message: appErr.ErrMissingUserWallet.Error(),
			},
		}

	case errors.Is(err, appErr.ErrAddingBeneficiary):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ADDING_BENEFICIARY_ERROR",
				Message: appErr.ErrAddingBeneficiary.Error(),
			},
		}

	case errors.Is(err, appErr.ErrMakingLoanRepayment):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ERROR_MAKING_REPAYMENT",
				Message: appErr.ErrMakingLoanRepayment.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingBeneficiaries):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ERROR_FETCHING_BENEFICIARIES",
				Message: appErr.ErrFetchingBeneficiaries.Error(),
			},
		}

	case errors.Is(err, appErr.ErrCreatingSavingsGoal):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "SAVINGS_GOAL_CREATION_ERROR",
				Message: appErr.ErrCreatingSavingsGoal.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingUserGoals):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FETCHING_USER_GOALS_ERROR",
				Message: appErr.ErrFetchingUserGoals.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingGoalSummary):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FETCHING_GOAL_SUMMARY_ERROR",
				Message: appErr.ErrFetchingGoalSummary.Error(),
			},
		}

	case errors.Is(err, appErr.ErrSMSDeliveryFailed):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "SMS_DELIVERY_FAILED",
				Message: appErr.ErrSMSDeliveryFailed.Error(),
			},
		}

	case errors.Is(err, appErr.ErrSMSServiceNotConfigured):
		return ErrorMapping{
			Status: http.StatusServiceUnavailable,
			Error: APIError{
				Code:    "SMS_SERVICE_NOT_CONFIGURED",
				Message: appErr.ErrSMSServiceNotConfigured.Error(),
			},
		}

	case errors.Is(err, appErr.ErrRequestingForCard):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "CARD_REQUEST_ERROR",
				Message: appErr.ErrRequestingForCard.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInsufficientBalance):
		return ErrorMapping{
			Status: http.StatusForbidden,
			Error: APIError{
				Code:    "INSUFFICIENT_FUNDS",
				Message: appErr.ErrInsufficientBalance.Error(),
			},
		}

	case errors.Is(err, appErr.ErrNewUserTransferRestriction):
		return ErrorMapping{
			Status: http.StatusForbidden,
			Error: APIError{
				Code:    "NEW_USER_TRANSFER_RESTRICTION",
				Message: appErr.ErrNewUserTransferRestriction.Error(),
			},
		}

	case errors.Is(err, appErr.ErrBVNWithFaceVerificationNotFound):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "BVN_FACE_VERIFICATION_NOT_FOUND",
				Message: "bvn with face verification not found or face did not match",
			},
		}

	case errors.Is(err, appErr.ErrValidatingBVNWithFace):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "BVN_FACE_MISMATCH",
				Message: "face verification did not match the provided bvn",
			},
		}

	case errors.Is(err, appErr.ErrNINWithFaceVerificationNotFound):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "NIN_FACE_VERIFICATION_NOT_FOUND",
				Message: "nin with face verification not found or face did not match",
			},
		}

	case errors.Is(err, appErr.ErrValidatingNINWithFace):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "NIN_FACE_MISMATCH",
				Message: "face verification did not match the provided nin",
			},
		}

	case errors.Is(err, appErr.ErrInvalidISPAmount):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_VAS_AMOUNT",
				Message: appErr.ErrInvalidISPAmount.Error(),
			},
		}

	case errors.Is(err, appErr.ErrGettingAirtime):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "FAILED_TO_GET_AIRTIME",
				Message: appErr.ErrGettingAirtime.Error(),
			},
		}

	case errors.Is(err, appErr.ErrVASAmbiguous):
		return ErrorMapping{
			Status: http.StatusRequestTimeout,
			Error: APIError{
				Code:    "REQUEST_TIMEOUT",
				Message: appErr.ErrVASAmbiguous.Error(),
			},
		}

	case errors.Is(err, appErr.ErrGettingData):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "FAILED_TO_GET_DATA",
				Message: appErr.ErrGettingData.Error(),
			},
		}

	case errors.Is(err, appErr.ErrValidatingElectricity):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "FAILED_TO_VALIDATE_ELECTRICITY",
				Message: appErr.ErrValidatingElectricity.Error(),
			},
		}

	case errors.Is(err, appErr.ErrPayingElectricityBill):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "FAILED_TO_PAY_ELECTRICITY_BILL",
				Message: appErr.ErrPayingElectricityBill.Error(),
			},
		}

	case errors.Is(err, appErr.ErrValidatingCable):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "FAILED_TO_VALIDATE_CABLE",
				Message: appErr.ErrValidatingCable.Error(),
			},
		}

	case errors.Is(err, appErr.ErrPayingCableBill):
		return ErrorMapping{
			Status: http.StatusBadGateway,
			Error: APIError{
				Code:    "FAILED_TO_PAY_CABLE_BILL",
				Message: appErr.ErrPayingCableBill.Error(),
			},
		}

	case errors.Is(err, appErr.ErrFetchingAllCategories):
		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "ALL_CATEGORIES_ERROR",
				Message: appErr.ErrFetchingAllCategories.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidPhoneNumber):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_PHONE_NUMBER",
				Message: appErr.ErrInvalidPhoneNumber.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidAccountNumber):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_ACCOUNT_NUMBER",
				Message: appErr.ErrInvalidAccountNumber.Error(),
			},
		}

	case errors.Is(err, appErr.ErrInvalidAccountType):
		return ErrorMapping{
			Status: http.StatusUnprocessableEntity,
			Error: APIError{
				Code:    "INVALID_ACCOUNT_TYPE",
				Message: appErr.ErrInvalidAccountType.Error(),
			},
		}

	case errors.Is(err, appErr.ErrEmailOutOfService):
		return ErrorMapping{
			Status: http.StatusServiceUnavailable,
			Error: APIError{
				Code:    "EMAIL_OUT_SERVICE",
				Message: appErr.ErrEmailOutOfService.Error(),
			},
		}

	case errors.Is(err, appErr.ErrProviderServiceUnavailable):
		return ErrorMapping{
			Status: http.StatusServiceUnavailable,
			Error: APIError{
				Code:    "SERVICE_UNAVAILABLE",
				Message: appErr.ErrProviderServiceUnavailable.Error(),
			},
		}

	default:
		var providerErr *appErr.XpressWalletProviderError
		if errors.As(err, &providerErr) {
			return ErrorMapping{
				Status: http.StatusUnprocessableEntity,
				Error: APIError{
					Code:    "XPRESS_WALLET_PROVIDER_ERROR",
					Message: providerErr.Message,
				},
			}
		}

		var termiiErr *appErr.TermiiError
		if errors.As(err, &termiiErr) {
			status := termiiErr.Code
			if status == 0 {
				status = http.StatusBadGateway // network error, timeout, marshal failure
			}
			return ErrorMapping{
				Status: status,
				Error: APIError{
					Code:    "SMS_PROVIDER_ERROR",
					Message: termiiErr.Message, // forward the actual Termii message
				},
			}
		}

		var zeptoErr *appErr.ZeptoError
		if errors.As(err, &zeptoErr) {
			// Infrastructure/config-side failures (expired credits, unverified
			// sender domain, rate limits, invalid token, transport errors) are not
			// user-actionable and would leak our setup — surface a generic message.
			if isZeptoInfraCode(zeptoErr.Code) || isZeptoInfraCode(zeptoErr.SubCode) {
				return ErrorMapping{
					Status: http.StatusServiceUnavailable,
					Error: APIError{
						Code:    "EMAIL_OUT_SERVICE",
						Message: appErr.ErrEmailOutOfService.Error(),
					},
				}
			}
			// Validation-side failures (bad payload, invalid recipient) — forward
			// the actual ZeptoMail message to aid debugging.
			status := zeptoErr.Status
			if status < 400 {
				status = http.StatusBadGateway // unknown / unexpected
			}
			return ErrorMapping{
				Status: status,
				Error: APIError{
					Code:    "EMAIL_PROVIDER_ERROR",
					Message: zeptoErr.Message,
				},
			}
		}

		return ErrorMapping{
			Status: http.StatusInternalServerError,
			Error: APIError{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An unexpected error occurred, please try again later",
			},
		}
	}
}

// zeptoInfraCodes are ZeptoMail error codes (and our synthetic transport codes)
// that represent infrastructure/config-side failures rather than a bad request
// payload — none are actionable by the end user, so they are surfaced as a
// generic "email out of service" message instead of forwarding provider detail.
var zeptoInfraCodes = map[string]struct{}{
	// Transport-level (synthesized in providers/email/zepto.go)
	"TIMEOUT":       {},
	"NETWORK_ERROR": {},
	"CLIENT_ERROR":  {},
	// ZeptoMail category codes
	"TM_3501": {}, // credits expired
	"TM_5001": {}, // credit exhausted
	"TM_3601": {}, // daily limit / trial limit / account blocked / IP not whitelisted
	"TM_4001": {}, // unverified sender domain / account not yet approved / invalid from
	// ZeptoMail sub-codes
	"SERR_157": {}, // sendmail token invalid
	"SERR_156": {}, // sending IP not whitelisted
	"LE_101":   {}, // credits expired
	"LE_102":   {}, // credit exhausted
	"SMI_115":  {}, // per-day limit exhausted
	"SM_133":   {}, // trial sending limit exceeded
	"SM_111":   {}, // sender address domain not verified
	"SM_128":   {}, // account not yet reviewed/approved
	"AE_101":   {}, // account blocked
}

func isZeptoInfraCode(code string) bool {
	if code == "" {
		return false
	}
	_, ok := zeptoInfraCodes[code]
	return ok
}
