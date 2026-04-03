package transaction

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewServie(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) FetchRecentTransactions(ctx context.Context, mobileUserID string) ([]TransactionResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)

	if mobileUserID == "" {
		return nil, errors.New("missing user id")
	}

	user, err := s.repo.FetchUserWithUserID(ctx, mobileUserID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	transactions, err := s.repo.FetchRecentTransactions(ctx, mobileUserID, user.WalletID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no transaction found")
		}
		return nil, err
	}

	result := make([]TransactionResponse, len(transactions))

	for i, t := range transactions {
		result[i] = TransactionResponse{
			ID:          t.ID,
			Type:        t.Type,
			Description: t.Description,
			Date:        t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Status:      t.Status,
			Amount:      t.Amount,
		}
	}

	return result, nil
}
