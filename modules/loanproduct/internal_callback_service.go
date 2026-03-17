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
	if status == LoanConvertedToLoan && coreLoanID == nil {
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
	case LoanStatusReviewed, LoanStatusApproved, LoanStatusRejected, LoanConvertedToLoan:
		return true
	default:
		return false
	}
}

func canTransition(from, to LoanStatus) bool {
	switch from {
	case LoanStatusPending:
		return to == LoanStatusReviewed || to == LoanStatusApproved || to == LoanStatusRejected
	case LoanStatusReviewed:
		return to == LoanStatusApproved || to == LoanStatusRejected
	case LoanStatusApproved:
		return to == LoanConvertedToLoan
	case LoanStatusRejected, LoanConvertedToLoan:
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
