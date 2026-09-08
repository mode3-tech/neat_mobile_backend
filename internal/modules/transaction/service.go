package transaction

import (
	"context"
	"errors"
	"log"
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
		log.Printf("transaction service: failed to fetch transaction id=%s: %v", txID, err)
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
		log.Printf("transaction service: failed to fetch recent transactions for user=%s: %v", mobileUserID, err)
		return nil, appErr.ErrFetchingTransactions
	}

	result := make([]TransactionResponse, len(transactions))

	for i, t := range transactions {
		result[i] = toTransactionResponse(t)
	}

	return result, nil
}

func (s *Service) AddTransaction(ctx context.Context, transaction *Transaction) error {
	if err := s.repo.AddTransaction(ctx, transaction); err != nil {
		log.Printf("transaction service: failed to add transaction id=%s: %v", transaction.ID, err)
		return err
	}
	return nil
}

func (s *Service) FetchTransactionsPaged(ctx context.Context, userID, cursor string, limit int) (*PagedTransactionResponse, error) {
	if limit < 0 || limit > 50 {
		limit = 20
	}

	var cursorTime time.Time
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339, cursor)
		if err != nil {
			return nil, appErr.ErrInvalidCursor
		}
		cursorTime = parsed
	}

	txs, err := s.repo.FetchTransactionPaged(ctx, userID, cursorTime, limit)
	if err != nil {
		log.Printf("transaction service: failed to fetch paged transactions for user=%s: %v", userID, err)
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
	if err := s.repo.AddTransaction(ctx, txn); err != nil {
		log.Printf("transaction service: failed to create transaction id=%s: %v", txn.ID, err)
		return err
	}
	return nil
}

func (s *Service) UpdateTransactionStatus(ctx context.Context, txID string, balanceAfter int64, status TransactionStatus) error {
	if err := s.repo.UpdateTransactionStatus(ctx, txID, balanceAfter, status); err != nil {
		log.Printf("transaction service: failed to update transaction id=%s to status=%s: %v", txID, status, err)
		return err
	}
	return nil
}

// toTransactionResponse maps a stored transaction to its API shape, including
// counterparty (sender for a credit, recipient for a debit), session id, and
// narration when present.
func toTransactionResponse(t Transaction) TransactionResponse {
	resp := TransactionResponse{
		ID:             t.ID,
		Type:           t.Type,
		Description:    t.Description,
		Reference:      t.Reference,
		Date:           t.CreatedAt.Format(time.RFC3339),
		Status:         t.Status,
		Amount:         float64(t.Amount) / 100,
		UsedCashback:   t.UsedCashback,
		CashbackAmount: float64(t.CashbackAmount) / 100,
		ActualAmount:   float64(t.Amount+t.CashbackAmount) / 100,
		Charges:        float64(t.Charges) / 100,
		VAT:            float64(t.VAT) / 100,
		SessionID:      t.SessionID,
		Narration:      t.Narration,
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
