package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/transaction"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type Service struct {
	repo           *Repository
	b2             UploadService
	notifier       *notification.Service
	pdfShiftAPIKey string
	deviceVerifier DeviceVerifier
}

func NewService(repo *Repository, b2 UploadService, notifier *notification.Service, pdfShiftAPIKey string, deviceVerifier DeviceVerifier) *Service {
	return &Service{repo: repo, b2: b2, notifier: notifier, pdfShiftAPIKey: pdfShiftAPIKey, deviceVerifier: deviceVerifier}
}

func (s *Service) GetAccountSummary(ctx context.Context, mobileUserID, deviceID string) (*AccountSummary, error) {
	if _, err := s.repo.GetDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, appErr.ErrUnauthorized
	}

	accountInfo, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	var loanBalance float64
	var activeLoans []ActiveLoan

	if accountInfo.CoreCustomerID != nil {
		loans, err := s.repo.GetLoansByCustomerID(ctx, *accountInfo.CoreCustomerID)
		if err == nil {
			for _, loan := range loans {
				loanBalance += loan.OutstandingBalance
				if strings.ToLower(loan.Status) != "active" {
					continue
				}
				activeLoans = append(activeLoans, ActiveLoan{
					LoanID:           loan.LoanID,
					LoanNumber:       loan.LoanNumber,
					LoanAmount:       loan.LoanAmount,
					TotalRepayment:   loan.OutstandingBalance,
					MonthlyRepayment: loan.NextPayment,
					NextDueDate:      loan.NextDueDate,
				})
			}
		}
	}

	if activeLoans == nil {
		activeLoans = []ActiveLoan{}
	}

	return &AccountSummary{
		FullName:               strings.TrimSpace(accountInfo.FirstName + " " + accountInfo.LastName),
		BankName:               accountInfo.BankName,
		Email:                  accountInfo.Email,
		BVN:                    accountInfo.BVN,
		DOB:                    accountInfo.DOB,
		ProfilePicture:         accountInfo.ProfilePicture,
		Address:                accountInfo.Address,
		PhoneNumber:            accountInfo.Phone,
		AccountNumber:          accountInfo.AccountNumber,
		AvailableBalance:       accountInfo.AvailableBalance,
		LoanBalance:            loanBalance,
		ActiveLoans:            activeLoans,
		IsNotificationsEnabled: accountInfo.IsNotificationsEnabled,
	}, nil
}

func (s *Service) RequestAccountStatement(ctx context.Context, mobileUserID, deviceID string, req AccountStatementRequest) (string, error) {
	if req.DateFrom.IsZero() {
		log.Printf("date_from is required")
		return "", appErr.ErrInvalidDateFrom
	}
	if req.DateTo.IsZero() {
		log.Printf("date_to is required")
		return "", appErr.ErrInvalidDateTo
	}
	now := time.Now().UTC()
	if req.DateFrom.After(now) {
		log.Printf("date_from cannot be in the future: %v", req.DateFrom)
		return "", appErr.ErrInvalidDateFrom
	}
	// if req.DateTo.After(now) {
	// 	log.Printf("date_to cannot be in the future: %v", req.DateTo)
	// 	return "", errors.New("date_to cannot be in the future")
	// }
	if !req.DateFrom.Before(req.DateTo) {
		log.Printf("invalid date range for account statement request: %v to %v", req.DateFrom, req.DateTo)
		return "", appErr.ErrInvalidDateRange
	}
	if req.DateTo.Sub(req.DateFrom) > 365*24*time.Hour {
		log.Printf("date range for account statement request exceeds 365 days: %v to %v", req.DateFrom, req.DateTo)
		return "", appErr.ErrInvalidDateRange
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		log.Printf("failed to verify device for account statement request: %v", err)
		return "", appErr.ErrUnauthorized
	}

	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		log.Printf("failed to retrieve user for account statement request: %v", err)
		return "", appErr.ErrUnauthorized
	}

	filePath := fmt.Sprintf("statements/%s_%s_%s_to_%s.%s", auth.TitleCase(user.FirstName), auth.TitleCase(user.LastName), req.DateFrom.Format("20060102"), req.DateTo.Format("20060102"), req.Format)

	job, err := s.repo.CreateAccountReportJob(ctx, &AccountReportJob{
		ID:           uuid.NewString(),
		MobileUserID: mobileUserID,
		WalletID:     user.WalletID,
		Type:         "account_statement",
		FilePath:     filePath,
		Status:       ReportStatusPending,
		DateFrom:     &req.DateFrom,
		DateTo:       &req.DateTo,
		Format:       req.Format,
	})
	if err != nil {
		log.Printf("failed to create account report job: %v", err)
		return "", fmt.Errorf("failed to create account report job: %w", err)
	}

	return job.ID, nil

}

func (s *Service) ProcessPendingStatementJobs(ctx context.Context) {
	jobs, err := s.repo.ClaimPendingAccountReportJobs(ctx, 1)
	if err != nil {
		return
	}

	for _, job := range jobs {
		s.ProcessStatementJob(ctx, job)
	}
}

func (s *Service) ClaimPendingStatementJobs(ctx context.Context, limit int) ([]AccountReportJob, error) {
	return s.repo.ClaimPendingAccountReportJobs(ctx, limit)
}

func (s *Service) ProcessStatementJob(ctx context.Context, job AccountReportJob) {
	if err := s.processAccountStatementRequest(ctx, job.FilePath, job.WalletID, job.MobileUserID, AccountStatementRequest{
		DateFrom: *job.DateFrom,
		DateTo:   *job.DateTo,
		Format:   job.Format,
	}); err != nil {
		s.repo.MarkJobFailed(ctx, job.ID, err.Error())
		return
	}

	s.repo.MarkJobReady(ctx, job.ID)

	s.notifier.SendToUser(
		ctx,
		job.MobileUserID,
		"Your statement is ready",
		"transaction",
		fmt.Sprintf("Your account statement for the period %s to %s is ready for download.",
			job.DateFrom.Format("2006-01-02"),
			job.DateTo.Format("2006-01-02")),
		map[string]any{
			"job_id": job.ID,
		},
	)
}

func (s *Service) GetStatementJobStatus(ctx context.Context, mobileUserID, deviceID, jobID string) (*AccountReportJob, string, error) {
	job, err := s.repo.GetAccountReportJob(ctx, jobID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve account report job: %w", err)
	}

	if job.MobileUserID != strings.TrimSpace(mobileUserID) {
		return nil, "", appErr.ErrNotFound
	}

	var downloadURL string
	if job.Status == ReportStatusReady && job.FilePath != "" {
		if job.DownloadURL == "" || job.URLExpiresAt == nil || time.Until(*job.URLExpiresAt) < 5*time.Minute {
			expiry := 15 * time.Minute
			downloadURL, err = s.b2.PresignURL(ctx, job.FilePath, expiry)
			if err != nil {
				return nil, "", fmt.Errorf("failed to generate download link for account statement: %w", err)
			}
			expiresAt := time.Now().Add(expiry)
			if err := s.repo.SaveDownloadURL(ctx, job.ID, downloadURL, expiresAt); err != nil {
				return nil, "", fmt.Errorf("failed to save download URL for account statement: %w", err)
			}
		} else {
			downloadURL = job.DownloadURL
		}
	}
	return job, downloadURL, nil
}

func (s *Service) GetLatestAccountStatement(ctx context.Context, mobileUserID, deviceID string) (*GetLatestAccountStatementResponse, error) {
	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	job, err := s.repo.GetLastestAccountStatement(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	var downloadURL string
	if job.Status == ReportStatusReady && job.FilePath != "" {
		if job.DownloadURL == "" || job.URLExpiresAt == nil || time.Until(*job.URLExpiresAt) < 5*time.Minute {
			expiry := 15 * time.Minute
			downloadURL, err = s.b2.PresignURL(ctx, job.FilePath, expiry)
			if err != nil {
				return nil, fmt.Errorf("failed to generate download link for account statement: %w", err)
			}
			expiresAt := time.Now().Add(expiry)
			if err := s.repo.SaveDownloadURL(ctx, job.ID, downloadURL, expiresAt); err != nil {
				return nil, fmt.Errorf("failed to save download URL for account statement: %w", err)
			}
		} else {
			downloadURL = job.DownloadURL
		}
	}

	return &GetLatestAccountStatementResponse{
		DownloadURL: downloadURL,
	}, nil
}

func (s *Service) processAccountStatementRequest(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	format := strings.TrimSpace(string(req.Format))

	switch format {
	case "pdf":
		if err := s.generatePDF(ctx, key, walletID, mobileUserID, req); err != nil {
			return fmt.Errorf("failed to generate account statement PDF: %w", err)
		}
		return nil
	case "xlsx":
		if err := s.generateXLSX(ctx, key, walletID, mobileUserID, req); err != nil {
			return fmt.Errorf("failed to generate account statement XLSX: %w", err)
		}
		return nil
	default:
		log.Printf("unsupported account statement format requested: %s", format)
		return appErr.ErrInvalidFileFormat
	}
}

func (s *Service) UpdateProfile(ctx context.Context, mobileUserID, deviceID string, profilePictureURL *string, req UpdateProfileRequest) error {
	data := UpdateProfileData{
		Address:           req.Address,
		Email:             req.Email,
		ProfilePictureURL: profilePictureURL,
	}

	if err := s.repo.UpdateProfile(ctx, mobileUserID, data); err != nil {
		return err
	}

	return nil
}

// func (s *Service) generateCSV(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
// 	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
// 	if err != nil {
// 		return fmt.Errorf("failed to retrieve transactions: %w", err)
// 	}

// 	var buf bytes.Buffer
// 	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel opens the file with correct encoding
// 	w := csv.NewWriter(&buf)

// 	w.Write([]string{"Time", "Date", "Description", "Debit (NGN)", "Credit (NGN)", "Balance Before (NGN)", "Balance After (NGN)", "Transaction Reference"})

// 	for _, tx := range transactions {
// 		debit, credit := "", ""
// 		amount := fmt.Sprintf("%.2f", float64(tx.Amount)/100)
// 		if tx.Type == transaction.TransactionTypeDebit {
// 			debit = amount
// 		} else {
// 			credit = amount
// 		}

// 		w.Write([]string{
// 			tx.CreatedAt.Format("Jan 02 2006 15:04:05"),
// 			tx.CreatedAt.Format("Jan 02 2006"),
// 			tx.Description,
// 			debit,
// 			credit,
// 			fmt.Sprintf("%.2f", float64(tx.BalanceBefore)/100),
// 			fmt.Sprintf("%.2f", float64(tx.BalanceAfter)/100),
// 			tx.Reference,
// 		})
// 	}
// 	w.Flush()

// 	if err := w.Error(); err != nil {
// 		return fmt.Errorf("failed to write transactions to csv: %w", err)
// 	}

// 	if err := s.b2.UploadDocument(ctx, key, bytes.NewReader(buf.Bytes()), "text/csv"); err != nil {
// 		return fmt.Errorf("failed to upload account statement to storage: %w", err)
// 	}

// 	return nil
// }

func (s *Service) generateXLSX(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
	if err != nil {
		return fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	account, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return fmt.Errorf("failed to retrieve account info: %w", err)
	}

	var totalDebits, totalCredits int64
	for _, tx := range transactions {
		if tx.Type == transaction.TransactionTypeDebit {
			totalDebits += tx.Amount
		} else {
			totalCredits += tx.Amount
		}
	}

	var openingBalance, closingBalance int64
	if len(transactions) > 0 {
		first := transactions[0]
		if first.Type == transaction.TransactionTypeDebit {
			openingBalance = first.BalanceAfter + first.Amount
		} else {
			openingBalance = first.BalanceAfter - first.Amount
		}
		closingBalance = transactions[len(transactions)-1].BalanceAfter
	}

	formatAmount := func(kobo int64) string {
		return fmt.Sprintf("%.2f", float64(kobo)/100)
	}

	f := excelize.NewFile()
	sheet := "Statement"
	f.SetSheetName("Sheet1", sheet)

	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	debitStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "C00000"}})
	creditStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "375623"}})

	summaryRows := [][]interface{}{
		{"Account Name", account.FirstName + " " + account.LastName},
		{"Account Number", account.AccountNumber},
		{"Period", req.DateFrom.Format("2 Jan 2006") + " – " + req.DateTo.Format("2 Jan 2006")},
		{"Opening Balance (NGN)", formatAmount(openingBalance)},
		{"Total Deposits (NGN)", formatAmount(totalCredits)},
		{"Total Withdrawals (NGN)", formatAmount(totalDebits)},
		{"Closing Balance (NGN)", formatAmount(closingBalance)},
		{"Generated", time.Now().Format("2 Jan 2006 15:04:05")},
	}
	for i, row := range summaryRows {
		rowNum := i + 1
		labelCell, _ := excelize.CoordinatesToCellName(1, rowNum)
		f.SetCellValue(sheet, labelCell, row[0])
		f.SetCellStyle(sheet, labelCell, labelCell, labelStyle)
		valCell, _ := excelize.CoordinatesToCellName(2, rowNum)
		f.SetCellValue(sheet, valCell, row[1])
	}

	headerRow := len(summaryRows) + 2
	headers := []string{"Date", "Time", "Description", "Reference", "Debit (NGN)", "Credit (NGN)", "Balance Before (NGN)", "Balance After (NGN)"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, headerRow)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	for i, tx := range transactions {
		row := headerRow + 1 + i
		debit, credit := "", ""
		amount := formatAmount(tx.Amount)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}

		values := []interface{}{
			tx.CreatedAt.Format("Jan 02 2006"),
			tx.CreatedAt.Format("15:04:05"),
			tx.Description,
			tx.Reference,
			debit,
			credit,
			formatAmount(tx.BalanceBefore),
			formatAmount(tx.BalanceAfter),
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, v)
		}

		debitCell, _ := excelize.CoordinatesToCellName(5, row)
		creditCell, _ := excelize.CoordinatesToCellName(6, row)
		if debit != "" {
			f.SetCellStyle(sheet, debitCell, debitCell, debitStyle)
		}
		if credit != "" {
			f.SetCellStyle(sheet, creditCell, creditCell, creditStyle)
		}
	}

	colWidths := map[string]float64{"A": 14, "B": 12, "C": 36, "D": 36, "E": 16, "F": 16, "G": 22, "H": 22}
	for col, width := range colWidths {
		f.SetColWidth(sheet, col, col, width)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return fmt.Errorf("failed to write xlsx: %w", err)
	}

	if err := s.b2.UploadDocument(ctx, key, bytes.NewReader(buf.Bytes()), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return fmt.Errorf("failed to upload xlsx to storage: %w", err)
	}

	return nil
}

func (s *Service) generatePDF(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	if s.pdfShiftAPIKey == "" {
		return errors.New("PDF generation is not configured")
	}

	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
	if err != nil {
		return fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	account, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return fmt.Errorf("failed to retrieve account info for PDF: %w", err)
	}

	// compute summary figures
	var totalDebits, totalCredits int64
	for _, tx := range transactions {
		if tx.Type == transaction.TransactionTypeDebit {
			totalDebits += tx.Amount
		} else {
			totalCredits += tx.Amount
		}
	}

	formatAmount := func(kobo int64) string {
		return fmt.Sprintf("%.2f", float64(kobo)/100)
	}

	var openingBalance, closingBalance int64
	if len(transactions) > 0 {
		first := transactions[0]
		if first.Type == transaction.TransactionTypeDebit {
			openingBalance = first.BalanceAfter + first.Amount
		} else {
			openingBalance = first.BalanceAfter - first.Amount
		}
		closingBalance = transactions[len(transactions)-1].BalanceAfter
	}

	// build transaction rows
	rows := make([]statementTxRow, 0, len(transactions))
	for _, tx := range transactions {
		debit, credit := "", ""
		amount := formatAmount(tx.Amount)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}
		rows = append(rows, statementTxRow{
			Date:        tx.CreatedAt.Format("02 Jan 2006 15:04"),
			Description: tx.Description,
			Reference:   tx.Reference,
			Debit:       debit,
			Credit:      credit,
			Balance:     formatAmount(tx.BalanceAfter),
		})
	}

	data := statementTemplateData{
		TodayDate:        time.Now().Format("02 Jan 2006"),
		StartDate:        req.DateFrom.Format("02 Jan 2006"),
		EndDate:          req.DateTo.Format("02 Jan 2006"),
		AccountName:      strings.TrimSpace(account.FirstName + " " + account.LastName),
		Address:          account.Address,
		AccountNumber:    account.AccountNumber,
		OpeningBalance:   formatAmount(openingBalance),
		TotalWithdrawals: formatAmount(totalDebits),
		TotalLodgement:   formatAmount(totalCredits),
		ClosingBalance:   formatAmount(closingBalance),
		Transactions:     rows,
	}

	// render HTML template
	tmpl, err := template.ParseFiles("templates/account_statement.html")
	if err != nil {
		return fmt.Errorf("failed to parse statement template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("failed to render statement template: %w", err)
	}

	// call PDFShift API
	payload, err := json.Marshal(map[string]any{
		"source":           htmlBuf.String(),
		"format":           "A4",
		"margin":           "20mm",
		"wait_for_network": true,
	})
	if err != nil {
		return fmt.Errorf("failed to build PDF request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pdfshift.io/v3/convert/pdf", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create PDF request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth("api", s.pdfShiftAPIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("PDF API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PDF API returned status %d: %s", resp.StatusCode, string(body))
	}

	pdfBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read PDF response: %w", err)
	}

	log.Printf("generatePDF: size=%d bytes for key=%s", len(pdfBytes), key)

	if err := s.b2.UploadDocument(ctx, key, bytes.NewReader(pdfBytes), "application/pdf"); err != nil {
		return fmt.Errorf("failed to upload account statement PDF to storage: %w", err)
	}

	return nil
}

func (s *Service) uploadProfilePicture(ctx context.Context, file multipart.File, header multipart.FileHeader, mobileUserID string) (string, error) {
	key := fmt.Sprintf("profile-pictures/%s/%s", mobileUserID, header.Filename)
	if err := s.b2.UploadProfilePicture(ctx, key, file, header.Header.Get("Content-Type")); err != nil {
		return "", err
	}

	url := s.b2.ProfilePictureURL(key)
	return url, nil
}
