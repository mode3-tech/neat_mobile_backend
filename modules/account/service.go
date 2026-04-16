package account

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/transaction"
	s3bucket "neat_mobile_app_backend/providers/s3_bucket"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
)

type Service struct {
	repo         *Repository
	loanProvider LoanProvider
	b2           *s3bucket.BackblazeClient
	notifier     *notification.Service
}

func NewService(repo *Repository, loanProvider LoanProvider, b2 *s3bucket.BackblazeClient, notifier *notification.Service) *Service {
	return &Service{repo: repo, loanProvider: loanProvider, b2: b2, notifier: notifier}
}

func (s *Service) GetAccountSummary(ctx context.Context, mobileUserID, deviceID string) (*AccountSummary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	if _, err := s.repo.GetDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	accountInfo, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	var loanBalance float64
	var activeLoans []ActiveLoan

	loans, err := s.loanProvider.GetAllLoans(ctx, mobileUserID)
	if err == nil {
		for _, loan := range loans {
			loanBalance += loan.OutstandingBalance
			if strings.ToLower(loan.Status) != "active" {
				continue
			}
			activeLoans = append(activeLoans, ActiveLoan{
				LoanID:           loan.LoanID,
				LoanNumber:       loan.LoanNumber,
				LoanAmount:       loan.PrincipalAmount,
				TotalRepayment:   loan.OutstandingBalance,
				MonthlyRepayment: loan.NextDueAmount,
				NextDueDate:      loan.NextDueDate,
			})
		}
	}

	if activeLoans == nil {
		activeLoans = []ActiveLoan{}
	}

	return &AccountSummary{
		FullName:               strings.TrimSpace(accountInfo.FirstName + " " + accountInfo.LastName),
		BankName:               accountInfo.BankName,
		BVN:                    accountInfo.BVN,
		DOB:                    accountInfo.DOB,
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
	if strings.TrimSpace(mobileUserID) == "" {
		log.Printf("mobile user ID is required")
		return "", errors.New("mobile user ID is required")
	}

	if strings.TrimSpace(deviceID) == "" {
		log.Printf("device ID is required")
		return "", errors.New("device ID is required")
	}

	if req.DateFrom.IsZero() {
		log.Printf("date_from is required")
		return "", errors.New("date_from is required")
	}
	if req.DateTo.IsZero() {
		log.Printf("date_to is required")
		return "", errors.New("date_to is required")
	}
	now := time.Now().UTC()
	if req.DateFrom.After(now) {
		log.Printf("date_from cannot be in the future: %v", req.DateFrom)
		return "", errors.New("date_from cannot be in the future")
	}
	// if req.DateTo.After(now) {
	// 	log.Printf("date_to cannot be in the future: %v", req.DateTo)
	// 	return "", errors.New("date_to cannot be in the future")
	// }
	if !req.DateFrom.Before(req.DateTo) {
		log.Printf("invalid date range for account statement request: %v to %v", req.DateFrom, req.DateTo)
		return "", errors.New("date_from must be before date_to")
	}
	if req.DateTo.Sub(req.DateFrom) > 365*24*time.Hour {
		log.Printf("date range for account statement request exceeds 365 days: %v to %v", req.DateFrom, req.DateTo)
		return "", errors.New("date range cannot exceed 365 days")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		log.Printf("failed to verify device for account statement request: %v", err)
		return "", fmt.Errorf("failed to verify device: %w", err)
	}

	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		log.Printf("failed to retrieve user for account statement request: %v", err)
		return "", fmt.Errorf("failed to retrieve user: %w", err)
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

func (s *Service) GetStatementJobStatus(ctx context.Context, mobileUserID, jobID string) (*AccountReportJob, string, error) {
	job, err := s.repo.GetAccountReportJob(ctx, jobID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve account report job: %w", err)
	}

	if job.MobileUserID != strings.TrimSpace(mobileUserID) {
		return nil, "", errors.New("job not found")
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

func (s *Service) processAccountStatementRequest(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	format := strings.TrimSpace(string(req.Format))

	switch format {
	case "csv":
		{
			if err := s.generateCSV(ctx, key, walletID, mobileUserID, req); err != nil {
				return fmt.Errorf("failed to generate account statement CSV: %w", err)
			}
			return nil
		}
	case "pdf":
		if err := s.generatePDF(ctx, key, walletID, mobileUserID, req); err != nil {
			return fmt.Errorf("failed to generate account statement PDF: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported account statement format: %s", format)
	}
}

func (s *Service) UpdateProfile(ctx context.Context, mobileUserID, deviceID string, req UpdateProfileRequest) error {
	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("user id is missing")
	}

	if err := s.repo.UpdateProfile(ctx, mobileUserID, req); err != nil {
		return err
	}

	return nil
}

func (s *Service) generateCSV(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
	if err != nil {
		return fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel opens the file with correct encoding
	w := csv.NewWriter(&buf)

	w.Write([]string{"Time", "Date", "Description", "Debit (NGN)", "Credit (NGN)", "Balance Before (NGN)", "Balance After (NGN)", "Transaction Reference"})

	for _, tx := range transactions {
		debit, credit := "", ""
		amount := fmt.Sprintf("%.2f", float64(tx.Amount)/100)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}

		w.Write([]string{
			tx.CreatedAt.Format("Jan 02 2006 15:04:05"),
			tx.CreatedAt.Format("Jan 02 2006"),
			tx.Description,
			debit,
			credit,
			fmt.Sprintf("%.2f", float64(tx.BalanceBefore)/100),
			fmt.Sprintf("%.2f", float64(tx.BalanceAfter)/100),
			tx.Reference,
		})
	}
	w.Flush()

	if err := w.Error(); err != nil {
		return fmt.Errorf("failed to write transactions to csv: %w", err)
	}

	if err := s.b2.Upload(ctx, key, bytes.NewReader(buf.Bytes()), "text/csv"); err != nil {
		return fmt.Errorf("failed to upload account statement to storage: %w", err)
	}

	return nil
}

func (s *Service) generatePDF(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
	if err != nil {
		return fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	account, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return fmt.Errorf("failed to retrieve account info for PDF: %w", err)
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()

	// header band
	pdf.SetFillColor(31, 78, 121)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(277, 12, "ACCOUNT STATEMENT", "", 1, "C", true, 0, "")
	pdf.Ln(4)

	// account info
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(45, 6, "Name:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, strings.TrimSpace(account.FirstName+" "+account.LastName), "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(45, 6, "Account Number:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, account.AccountNumber, "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(45, 6, "Period:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, req.DateFrom.Format("Jan 02, 2006")+" - "+req.DateTo.Format("Jan 02, 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(5)

	// table columns
	type col struct {
		label string
		width float64
		align string
	}
	cols := []col{
		{"Date", 38, "L"},
		{"Description", 80, "L"},
		{"Reference", 55, "L"},
		{"Debit (NGN)", 30, "R"},
		{"Credit (NGN)", 30, "R"},
		{"Balance (NGN)", 34, "R"},
	}

	// table header
	pdf.SetFillColor(31, 78, 121)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 9)
	for _, c := range cols {
		pdf.CellFormat(c.width, 8, c.label, "1", 0, c.align, true, 0, "")
	}
	pdf.Ln(-1)

	// table rows
	pdf.SetFont("Arial", "", 8)
	fill := false
	for _, tx := range transactions {
		debit, credit := "", ""
		amount := fmt.Sprintf("%.2f", float64(tx.Amount)/100)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}

		if fill {
			pdf.SetFillColor(235, 242, 250)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(0, 0, 0)

		rowVals := []struct {
			val   string
			width float64
			align string
		}{
			{tx.CreatedAt.Format("Jan 02 2006 15:04"), cols[0].width, cols[0].align},
			{tx.Description, cols[1].width, cols[1].align},
			{tx.Reference, cols[2].width, cols[2].align},
			{debit, cols[3].width, cols[3].align},
			{credit, cols[4].width, cols[4].align},
			{fmt.Sprintf("%.2f", float64(tx.BalanceAfter)/100), cols[5].width, cols[5].align},
		}
		for _, cell := range rowVals {
			pdf.CellFormat(cell.width, 7, cell.val, "B", 0, cell.align, fill, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}

	// footer
	pdf.Ln(6)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(0, 6, "System generated statement — no signature required.", "", 0, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}

	log.Printf("generatePDF: size=%d bytes for key=%s", buf.Len(), key)

	if err := s.b2.Upload(ctx, key, bytes.NewReader(buf.Bytes()), "application/pdf"); err != nil {
		return fmt.Errorf("failed to upload account statement PDF to storage: %w", err)
	}

	return nil
}
