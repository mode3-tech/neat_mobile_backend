package transaction

type FetchAllTransactionsQuery struct {
	Limit string `form:"limit" binding:"required"`
	Page  string `form:"page" binding:"required"`
}

// type TransactionResponse struct {
// 	ID string `json:"id" binding:"required"`
// 	Type
// }
