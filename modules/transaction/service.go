package transaction

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewServie(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) FetchTransactions(ctx context.Context, mobileUserID, walletID string) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	walletID = strings.TrimSpace(walletID)

	if mobileUserID == "" {
		// return
	}
}
