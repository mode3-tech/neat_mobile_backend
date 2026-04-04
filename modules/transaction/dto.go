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
	Status     bool                 `json:"status"`
	Sections   []TransactionSection `json:"sections"`
	NextCursor string               `json:"next_cursor"` // empty when no more pages
	HasMore    bool                 `json:"has_more"`
}

type TransactionResponse struct {
	ID          string            `json:"id" binding:"required"`
	Type        TransactionType   `json:"type" binding:"required"`
	Description string            `json:"description" binding:"required"`
	Reference   string            `json:"reference" binding:"required"`
	Date        string            `json:"date" binding:"required"`
	Status      TransactionStatus `json:"status" binding:"status"`
	Amount      int64             `json:"amount" binding:"required"`
}
