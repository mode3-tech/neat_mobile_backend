package transaction

type FetchAllTransactionsQuery struct {
	Limit  int    `form:"limit" binding:"required"`
	Cursor string `form:"cursor"` // RFC3339 timestamp; empty = first page
}

type TransactionSection struct {
	Month        string                `json:"month"` // e.g. "April 2026"
	Transactions []TransactionResponse `json:"transactions"`
}

type PagedTransactionResponse struct {
	Sections   []TransactionSection `json:"sections"`
	NextCursor string               `json:"next_cursor"` // empty when no more pages
	HasMore    bool                 `json:"has_more"`
}

type Counterparty struct {
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	Bank          string `json:"bank"`
}

type TransactionResponse struct {
	ID          string            `json:"id" binding:"required"`
	Type        TransactionType   `json:"type" binding:"required"`
	Description string            `json:"description" binding:"required"`
	Reference   string            `json:"reference" binding:"required"`
	Date        string            `json:"date" binding:"required"`
	Status      TransactionStatus `json:"status" binding:"status"`
	// Amount in naira. Stored as kobo; whole naira = kobo/100, trailing kobo =
	// kobo%100 (e.g. 500050 kobo -> 5000.50).
	Amount float64 `json:"amount"`
	// SessionID is the interbank (NIP) session id, when available.
	SessionID string `json:"session_id,omitempty"`
	// Narration is the transfer note, when available.
	Narration *string `json:"narration,omitempty"`
	// Counterparty is the other party: the sender for a credit, the recipient
	// for a debit. Omitted when the transaction has no counterparty (e.g. loans).
	Counterparty *Counterparty `json:"counterparty,omitempty"`
}
