package transaction

type FetchTransactionsQuery struct {
	Limit string `form:"limit" binding:"required"`
	Page  string `form:"page" binding:"required"`
}
