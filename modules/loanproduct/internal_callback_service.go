package loanproduct

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InternalService struct {
	repo *InternalRepository
}

func NewInternalService(repo *InternalRepository) *InternalService {
	return &InternalService{repo: repo}
}

var (
	ErrBadRequest          = errors.New("bad request")
	ErrInvalidStatus       = errors.New("invalid status")
	ErrInvalidTransition   = errors.New("invalid loan status transition")
	ErrApplicationNotFound = errors.New("loan application not found")
)

func (s *InternalService) GetLoanApplicationsForCBA(ctx context.Context) (*GetLoanApplicationsForCBAResponse, error) {
	rows, err := s.repo.ListLoanApplicationsForCBA(ctx)
	if err != nil {
		return nil, err
	}

	resp := &GetLoanApplicationsForCBAResponse{
		Count:        len(rows),
		Applications: make([]CBAListLoanApplicationItem, 0, len(rows)),
	}

	for _, row := range rows {
		coreCustomerID := row.ApplicationCoreCustomerID
		if coreCustomerID == nil {
			coreCustomerID = row.UserCoreCustomerID
		}

		item := CBAListLoanApplicationItem{
			ApplicationRef: row.ApplicationRef,
			Loan: CBALoanApplicationReadDTO{
				ApplicationRef:  row.ApplicationRef,
				MobileUserID:    row.MobileUserID,
				CoreCustomerID:  coreCustomerID,
				PhoneNumber:     row.PhoneNumber,
				LoanProductType: row.LoanProductType,
				BusinessAddress: row.BusinessAddress,
				BusinessValue:   row.BusinessValue,
				BusinessType:    row.BusinessType,
				RequestedAmount: row.RequestedAmount,
				LoanStatus:      row.LoanStatus,
				Tenure:          row.Tenure,
				TenureValue:     row.TenureValue,
			},
		}

		if row.BVNRecordID != nil {
			item.BVNRecord = &CBABVNRecordReadDTO{
				BVN:                    valueOrEmpty(row.BVN),
				FirstName:              valueOrEmpty(row.FirstName),
				MiddleName:             valueOrEmpty(row.MiddleName),
				LastName:               valueOrEmpty(row.LastName),
				Gender:                 valueOrEmpty(row.Gender),
				Nationality:            valueOrEmpty(row.Nationality),
				StateOfOrigin:          valueOrEmpty(row.StateOfOrigin),
				DateOfBirth:            formatDatePtr(row.DateOfBirth),
				EmailAddress:           valueOrEmpty(row.EmailAddress),
				MobilePhone:            valueOrEmpty(row.MobilePhone),
				AlternativeMobilePhone: row.AlternativeMobilePhone,
				BankName:               valueOrEmpty(row.BankName),
				FullHomeAddress:        valueOrEmpty(row.FullHomeAddress),
				PassportOnBVN:          valueOrEmpty(row.PassportOnBVN),
				City:                   row.City,
				Landmark:               row.Landmark,
			}
		}

		resp.Applications = append(resp.Applications, item)
	}

	return resp, nil
}

func (s *InternalService) ApplyCBAStatusUpdate(ctx context.Context, applicationRef string, req UpdateLoanApplicationStatusRequest, rawPayload []byte) error {
	applicationRef = strings.TrimSpace(applicationRef)
	if applicationRef == "" || strings.TrimSpace(req.EventID) == "" {
		return ErrBadRequest
	}

	status := LoanStatus(strings.TrimSpace(req.Status))
	if !isAllowedCallbackStatus(status) {
		return ErrInvalidStatus
	}

	coreLoanID := trimmedPtr(req.CoreLoanID)
	if status == LoanStatusActive && coreLoanID == nil {
		return errors.New("core_loan_id is required when status is converted_to_loan")
	}

	now := time.Now().UTC()

	return s.repo.WithTx(ctx, func(repo *InternalRepository) error {
		app, err := repo.GetApplicationByRefForUpdate(ctx, applicationRef)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		if err != nil {
			return err
		}

		if app.LoanStatus != status && !canTransition(app.LoanStatus, status) {
			return ErrInvalidTransition
		}

		created, err := repo.InsertStatusEvent(ctx, &LoanApplicationStatusEvent{
			ID:             uuid.NewString(),
			EventID:        strings.TrimSpace(req.EventID),
			ApplicationRef: applicationRef,
			Status:         status,
			CoreLoanID:     coreLoanID,
			RawPayload:     string(rawPayload),
			ProcessedAt:    now,
		})
		if err != nil || !created {
			return err
		}

		if app.LoanStatus == status && sameStringPtr(app.CoreLoanID, coreLoanID) {
			return nil
		}

		return repo.UpdateApplicationStatus(ctx, applicationRef, status, coreLoanID, now)
	})
}

func isAllowedCallbackStatus(s LoanStatus) bool {
	switch s {
	case LoanStatusApproved, LoanStatusDeclined, LoanStatusActive:
		return true
	default:
		return false
	}
}

func canTransition(from, to LoanStatus) bool {
	switch from {
	case LoanStatusPending:
		return to == LoanStatusApproved || to == LoanStatusDeclined
	case LoanStatusApproved:
		return to == LoanStatusActive
	case LoanStatusDeclined, LoanStatusActive:
		return false
	default:
		return false
	}
}

func trimmedPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func sameStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.TrimSpace(*a) == strings.TrimSpace(*b)
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func formatDatePtr(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.Format("2006-01-02")
}
