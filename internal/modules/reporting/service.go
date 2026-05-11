package reporting

import (
	"context"
	"errors"
	"math"
	"strings"
)

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListSignedUsers(ctx context.Context, page, limit int) (*ListSignedUsersResponse, error) {
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := (page - 1) * limit

	rows, total, err := s.repo.ListSignedUsers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	users := make([]SignedUserItem, 0, len(rows))
	for _, r := range rows {
		item := SignedUserItem{
			MobileUserID:   r.ID,
			FirstName:      r.FirstName,
			LastName:       r.LastName,
			MiddleName:     r.MiddleName,
			Email:          r.Email,
			Phone:          r.Phone,
			BVN:            r.BVN,
			CoreCustomerID: r.CoreCustomerID,
			CustomerStatus: r.CustomerStatus,
			Username:       r.Username,
			RegisteredAt:   r.CreatedAt,
			Verified: VerifiedFlags{
				BVN:   r.IsBVNVerified,
				NIN:   r.IsNINVerified,
				Phone: r.IsPhoneVerified,
			},
		}

		if r.LoanStatus != nil && r.LastLoanAppliedAt != nil {
			item.LatestLoan = &LatestLoanSummary{
				Status:    *r.LoanStatus,
				AppliedAt: *r.LastLoanAppliedAt,
			}
		}

		users = append(users, item)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &ListSignedUsersResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) GetUserTransactions(ctx context.Context, mobileUserID string, limit, page int) (*UserTransactionResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("missing user id")
	}
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := (page - 1) * limit

	transactions, total, err := s.repo.GetTransactionsWithMobileUserID(ctx, mobileUserID, limit, offset)
	if err != nil {
		return nil, errors.New("an error occured when trying to fetch user transactions")
	}

	txs := make([]UserTransaction, 0, len(transactions))
	for _, r := range transactions {
		txs = append(txs, UserTransaction{
			MobileUserID:         r.MobileUserID,
			Type:                 r.Type,
			Amount:               float64(r.Amount) / 100,
			Charges:              float64(r.Charges),
			VAT:                  float64(r.VAT),
			BalanceBefore:        float64(r.BalanceBefore) / 100,
			BalanceAfter:         float64(r.BalanceAfter) / 100,
			TransactionReference: r.TransactionReference,
			Narration:            r.Narration,
			RecipientName:        r.RecipientName,
			RecipientAccount:     r.RecipientAccount,
			RecipientBank:        r.RecipientBank,
			Status:               r.Status,
			CreatedAt:            r.CreatedAt,
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &UserTransactionResponse{
		Transactions: txs,
		Total:        total,
		Page:         page,
		Limit:        limit,
		TotalPages:   totalPages,
	}, nil
}
