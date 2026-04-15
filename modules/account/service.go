package account

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"html/template"
	"log"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/transaction"
	s3bucket "neat_mobile_app_backend/providers/s3_bucket"
	apptemplates "neat_mobile_app_backend/templates"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
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
		FullName:         strings.TrimSpace(accountInfo.FirstName + " " + accountInfo.LastName),
		BankName:         accountInfo.BankName,
		BVN:              accountInfo.BVN,
		Address:          accountInfo.Address,
		PhoneNumber:      accountInfo.Phone,
		AccountNumber:    accountInfo.AccountNumber,
		AvailableBalance: accountInfo.AvailableBalance,
		LoanBalance:      loanBalance,
		ActiveLoans:      activeLoans,
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
	jobs, err := s.repo.GetPendingAccountReportJobs(ctx)
	if err != nil {
		return
	}

	for _, job := range jobs {

		if err := s.repo.MarkJobProcessing(ctx, job.ID); err != nil {
			fmt.Printf("failed to mark account report job %s as processing: %v\n", job.ID, err)
			continue
		}

		if err := s.processAccountStatementRequest(ctx, job.FilePath, job.WalletID, job.MobileUserID, AccountStatementRequest{
			DateFrom: *job.DateFrom,
			DateTo:   *job.DateTo,
			Format:   job.Format,
		}); err != nil {
			s.repo.MarkJobFailed(ctx, job.ID, err.Error())
			continue
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

type statementTemplateData struct {
	AccountName   string
	AccountNumber string
	StartDate     string
	EndDate       string
	Transactions  []statementRow
}

type statementRow struct {
	Date        string
	Description string
	Reference   string
	Debit       string
	Credit      string
	Balance     string
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

	rows := make([]statementRow, 0, len(transactions))
	for _, tx := range transactions {
		debit, credit := "", ""
		amount := fmt.Sprintf("%.2f", float64(tx.Amount)/100)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}
		rows = append(rows, statementRow{
			Date:        tx.CreatedAt.Format("Jan 02, 2006 15:04"),
			Description: tx.Description,
			Reference:   tx.Reference,
			Debit:       debit,
			Credit:      credit,
			Balance:     fmt.Sprintf("%.2f", float64(tx.BalanceAfter)/100),
		})
	}

	tmpl, err := template.ParseFS(apptemplates.FS, "account_statement.html")
	if err != nil {
		return fmt.Errorf("failed to parse statement template: %w", err)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, statementTemplateData{
		AccountName:   strings.TrimSpace(account.FirstName + " " + account.LastName),
		AccountNumber: account.AccountNumber,
		StartDate:     req.DateFrom.Format("Jan 02, 2006"),
		EndDate:       req.DateTo.Format("Jan 02, 2006"),
		Transactions:  rows,
	}); err != nil {
		return fmt.Errorf("failed to render statement template: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	url := "data:text/html;base64," + encoded

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var pdfBuf []byte

	if err = chromedp.Run(browserCtx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to generate PDF: %w", err)
			}
			pdfBuf = buf
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create PDF with chromedp: %w", err)
	}

	log.Printf("generatePDF: pdfBuf size=%d bytes for key=%s", len(pdfBuf), key)
	if len(pdfBuf) == 0 {
		return fmt.Errorf("chromedp produced empty PDF buffer for key=%s", key)
	}

	if err := s.b2.Upload(ctx, key, bytes.NewReader(pdfBuf), "application/pdf"); err != nil {
		return fmt.Errorf("failed to upload account statement PDF to storage: %w", err)
	}

	return nil
}
