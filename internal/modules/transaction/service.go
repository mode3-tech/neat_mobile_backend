package transaction

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewServie(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) FetchTransactionByID(ctx context.Context, txID string) (*TransactionResponse, error) {
	tx, err := s.repo.FetchTransactionByID(ctx, txID)
	if err != nil {
		return nil, appErr.ErrFetchingTransactions
	}
	resp := toTransactionResponse(*tx)
	return &resp, nil
}

func (s *Service) FetchRecentTransactions(ctx context.Context, mobileUserID string) ([]TransactionResponse, error) {
	transactions, err := s.repo.FetchRecentTransactions(ctx, mobileUserID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrNoTransactionsFound
		}
		return nil, appErr.ErrFetchingTransactions
	}

	result := make([]TransactionResponse, len(transactions))

	for i, t := range transactions {
		result[i] = toTransactionResponse(t)
	}

	return result, nil
}

func (s *Service) FetchTransactionsPaged(ctx context.Context, userID, cursor string, limit int) (*PagedTransactionResponse, error) {
	if limit < 0 || limit > 50 {
		limit = 20
	}

	var cursorTime time.Time
	if cursor != "" {
		_, err := time.Parse(time.RFC3339, cursor)
		if err != nil {
			return nil, appErr.ErrInvalidCursor
		}
	}

	txs, err := s.repo.FetchTransactionPaged(ctx, userID, cursorTime, limit)
	if err != nil {
		return nil, appErr.ErrFetchingTransactions
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
		Sections:   groupByMonth(txs),
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *Service) CreateTransaction(ctx context.Context, txn *Transaction) error {
	return s.repo.AddTransaction(ctx, txn)
}

func (s *Service) UpdateTransactionStatus(ctx context.Context, txID string, balanceAfter int64, status TransactionStatus) error {
	return s.repo.UpdateTransactionStatus(ctx, txID, balanceAfter, status)
}

// toTransactionResponse maps a stored transaction to its API shape, including
// counterparty (sender for a credit, recipient for a debit), session id, and
// narration when present.
func toTransactionResponse(t Transaction) TransactionResponse {
	resp := TransactionResponse{
		ID:          t.ID,
		Type:        t.Type,
		Description: t.Description,
		Reference:   t.Reference,
		Date:        t.CreatedAt.Format(time.RFC3339),
		Status:      t.Status,
		Amount:      float64(t.Amount) / 100, // kobo -> naira, keeping trailing kobo as decimals
		SessionID:   t.SessionID,
		Narration:   t.Narration,
	}
	if t.CounterpartyName != "" || t.CounterpartyAccount != "" || t.CounterpartyBank != "" {
		resp.Counterparty = &Counterparty{
			Name:          t.CounterpartyName,
			AccountNumber: t.CounterpartyAccount,
			Bank:          t.CounterpartyBank,
		}
	}
	return resp
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
		groups[k] = append(groups[k], toTransactionResponse(t))
	}

	sections := make([]TransactionSection, len(order))
	for i, k := range order {
		label := time.Date(k.year, k.month, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
		sections[i] = TransactionSection{Month: label, Transactions: groups[k]}
	}
	return sections
}
