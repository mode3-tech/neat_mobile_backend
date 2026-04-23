package wallet

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

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
