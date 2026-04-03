package transaction

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewServie(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) FetchRecentTransactions(ctx context.Context, mobileUserID, walletID string) (*TransactionResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	walletID = strings.TrimSpace(walletID)

	if mobileUserID == "" {
		return nil, errors.New("missing user id")
	}

	if walletID == "" {
		return nil, errors.New("missing wallet id")
	}

	return nil, nil
}
