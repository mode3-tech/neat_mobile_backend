package wallet

import "errors"

var (
	ErrWrongTransactionPin  = errors.New("wrong transaction pin")
	ErrTransactionPinLocked = errors.New("transaction pin locked")
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
