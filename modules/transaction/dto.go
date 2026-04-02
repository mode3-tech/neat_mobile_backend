package transaction

type FetchAllTransactionsQuery struct {
	Limit string `form:"limit" binding:"required"`
	Page  string `form:"page" binding:"required"`
}

type TransactionResponse struct {
	ID          string            `json:"id" binding:"required"`
	Type        TransactionType   `json:"type" binding:"required"`
	Description string            `json:"description" binding:"required"`
	Date        string            `json:"date" binding:"required"`
	Status      TransactionStatus `json:"status" binding:"status"`
	Amount      int64             `json:"amount" binding:"required"`
}
