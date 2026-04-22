package wallet

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxPinAttempts  = 5
	pinLockDuration = 30 * time.Minute
)

func (s *Service) verifyTransactionPin(ctx context.Context, mobileUserID, pin string) error {
	user, err := s.repo.GetUserForPinVerification(ctx, mobileUserID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	if user.TransactionPinLockedUntil != nil && user.TransactionPinLockedUntil.After(time.Now().UTC()) {
		return ErrTransactionPinLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(pin)); err != nil {
		newAttempts := user.FailedTransactionPinAttempts + 1
		if newAttempts >= maxPinAttempts {
			_ = s.repo.LockTransactionPin(ctx, mobileUserID, time.Now().UTC().Add(pinLockDuration))
			return ErrTransactionPinLocked
		}
		_ = s.repo.IncrementFailedPinAttempts(ctx, mobileUserID)
		return fmt.Errorf("%w: you have %d attempt(s) left", ErrWrongTransactionPin, maxPinAttempts-newAttempts)
	}

	_ = s.repo.ResetPinAttempts(ctx, mobileUserID)
	return nil
}

func parseCSV(reader io.Reader) ([]BulkTransferRecipientInfo, error) {
	r := csv.NewReader(reader)

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	recipients := make([]BulkTransferRecipientInfo, 0, len(rows)-1)
	for i, row := range rows[1:] { // skip header
		if len(row) < 4 {
			return nil, fmt.Errorf("row %d: expected at least 4 columns (amount, sort_code, account_number, account_name)", i+2)
		}

		amount, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount: %w", i+2, err)
		}

		recipient := BulkTransferRecipientInfo{
			Amount:        amount,
			SortCode:      strings.TrimSpace(row[1]),
			AccountNumber: strings.TrimSpace(row[2]),
		}

		accountName := strings.TrimSpace(row[3])
		recipient.AccountName = &accountName

		if len(row) >= 5 {
			narration := strings.TrimSpace(row[4])
			recipient.Narration = &narration
		}

		recipients = append(recipients, recipient)
	}

	return recipients, nil
}

func parseExcel(reader io.Reader) ([]BulkTransferRecipientInfo, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file must have a header row and at least one data row")
	}

	recipients := make([]BulkTransferRecipientInfo, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if len(row) < 4 {
			return nil, fmt.Errorf("row %d: expected at least 4 columns (amount, sort_code, account_number, account_name)", i+2)
		}

		amount, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount: %w", i+2, err)
		}

		recipient := BulkTransferRecipientInfo{
			Amount:        amount,
			SortCode:      strings.TrimSpace(row[1]),
			AccountNumber: strings.TrimSpace(row[2]),
		}

		accountName := strings.TrimSpace(row[3])
		recipient.AccountName = &accountName

		if len(row) >= 5 {
			narration := strings.TrimSpace(row[4])
			recipient.Narration = &narration
		}

		recipients = append(recipients, recipient)
	}

	return recipients, nil
}
