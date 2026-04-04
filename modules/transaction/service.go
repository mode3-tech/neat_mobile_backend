package transaction

import (
	"context"
	"errors"
	"strings"
	"time"

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
			Reference:   t.Reference,
			Status:      t.Status,
			Amount:      t.Amount,
		}
	}

	return result, nil
}

func (s *Service) FetchTransactionsPaged(ctx context.Context, userID, cursor string, limit int) (*PagedTransactionResponse, error) {
	if limit < 0 || limit > 50 {
		limit = 20
	}

	user, err := s.repo.FetchUserWithUserID(ctx, userID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	var cursorTime time.Time
	if cursor != "" {
		cursorTime, err = time.Parse(time.RFC3339, cursor)
		if err != nil {
			return nil, errors.New("invalid cursor format")
		}
	}

	txs, err := s.repo.FetchTransactionPaged(ctx, userID, user.WalletID, cursorTime, limit)
	if err != nil {
		return nil, err
	}

	hasMore := len(txs) > limit
	if hasMore {
		txs = txs[:limit] // trim the extra one
	}

	var nextCursor string
	if hasMore {
		nextCursor = txs[len(txs)-1].CreatedAt.Format(time.RFC3339)
	}

	return &PagedTransactionResponse{
		Status:     true,
		Sections:   groupByMonth(txs),
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// groupByMonth preserves DESC order since txs is already sorted that way.
func groupByMonth(txs []Transaction) []TransactionSection {
	type key struct {
		year  int
		month time.Month
	}

	var order []key
	groups := map[key][]TransactionResponse{}

	for _, t := range txs {
		k := key{t.CreatedAt.Year(), t.CreatedAt.Month()}
		if _, exists := groups[k]; !exists {
			order = append(order, k)
		}
		groups[k] = append(groups[k], TransactionResponse{
			ID:          t.ID,
			Type:        t.Type,
			Description: t.Description,
			Reference:   t.Reference,
			Date:        t.CreatedAt.Format(time.RFC3339),
			Status:      t.Status,
			Amount:      t.Amount,
		})
	}

	sections := make([]TransactionSection, len(order))
	for i, k := range order {
		label := time.Date(k.year, k.month, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
		sections[i] = TransactionSection{Month: label, Transactions: groups[k]}
	}
	return sections
}
