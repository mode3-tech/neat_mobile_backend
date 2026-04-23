package wallet

import (
	"errors"
	"neat_mobile_app_backend/internal/pinverifier"
)

var (
	ErrWrongTransactionPin      = pinverifier.ErrWrongTransactionPin
	ErrTransactionPinLocked     = pinverifier.ErrTransactionPinLocked
	ErrInvalidTransferRequest   = errors.New("invalid transfer request")
	ErrWalletNotFound           = errors.New("wallet not found")
	ErrDeviceVerificationFailed = errors.New("device verification failed")
	ErrTransferProviderFailed   = errors.New("transfer provider failed")
)

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusFailed    TransferStatus = "failed"
	TransferStatusCancelled TransferStatus = "cancelled"
)

type TransferType string

const (
	TransferTypeDebit  TransferType = "debit"
	TransferTypeCredit TransferType = "credit"
)

type ExpectedDepositStatus string

const (
	ExpectedDepositStatusPending   ExpectedDepositStatus = "pending"
	ExpectedDepositStatusExpired   ExpectedDepositStatus = "expired"
	ExpectedDepositStatusFulfilled ExpectedDepositStatus = "fulfilled"
)
