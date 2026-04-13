package account

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/transaction"
	s3bucket "neat_mobile_app_backend/providers/s3_bucket"
	"strings"
	"time"
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
		AccountNumber:    accountInfo.AccountNumber,
		AvailableBalance: accountInfo.AvailableBalance,
		WalletID:         accountInfo.InternalWalletID,
		LoanBalance:      loanBalance,
		ActiveLoans:      activeLoans,
	}, nil
}

func (s *Service) RequestAccountStatement(ctx context.Context, mobileUserID, deviceID string, req AccountStatementRequest) (string, error) {
	if strings.TrimSpace(mobileUserID) == "" {
		return "", errors.New("mobile user ID is required")
	}

	if strings.TrimSpace(deviceID) == "" {
		return "", errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return "", fmt.Errorf("failed to verify device: %w", err)
	}

	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve user: %w", err)
	}

	job, err := s.repo.CreateAccountReportJob(ctx, &AccountReportJob{
		MobileUserID: mobileUserID,
		WalletID:     user.WalletID,
		Type:         "account_statement",
		Status:       "pending",
		DateFrom:     &req.DateFrom,
		DateTo:       &req.DateTo,
		Format:       req.Format,
	})
	if err != nil {
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

		if err := s.processAccountStatementRequest(ctx, job.ID, job.WalletID, job.MobileUserID, AccountStatementRequest{
			DateFrom: *job.DateFrom,
			DateTo:   *job.DateTo,
			Format:   job.Format,
		}); err != nil {
			s.repo.MarkJobFailed(ctx, job.ID, err.Error())
			continue
		}

		s.repo.MarkJobReady(ctx, job.ID, job.FilePath)

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
	if job.Status != ReportStatusReady && job.FilePath != "" {
		downloadURL, err = s.b2.PresignURL(ctx, job.FilePath, 15*time.Minute)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate download link for account statement: %w", err)
		}
	}

	return job, downloadURL, nil
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

func (s *Service) processAccountStatementRequest(ctx context.Context, key, walletID, mobileUserID string, req AccountStatementRequest) error {
	transactions, err := s.repo.GetStatementTransactions(ctx, mobileUserID, walletID, req.DateFrom, req.DateTo)
	if err != nil {
		return fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	w.Write([]string{"Time", "Date", "Description", "Debit", "Credit", "Balance After(₦)", "Transasction Reference"})

	for _, tx := range transactions {
		debit, credit := "", ""
		amount := fmt.Sprintf("%.2f", float64(tx.Amount)/100)
		if tx.Type == transaction.TransactionTypeDebit {
			debit = amount
		} else {
			credit = amount
		}

		w.Write([]string{
			tx.CreatedAt.Format("2006-01-02 15:04:05"),
			tx.CreatedAt.Format("2006-01-02"),
			tx.Description,
			debit,
			credit,
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
