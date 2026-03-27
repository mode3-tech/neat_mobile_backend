package loanproduct

type UpdateLoanApplicationStatusRequest struct {
	EventID    string `json:"event_id" binding:"required"`
	Status     string `json:"status" binding:"required"`
	CoreLoanID string `json:"core_loan_id"`
}

type UpdateCustomerStatusRequest struct {
	EventID string `json:"event_id" binding:"required"`
	Status  string `json:"status" binding:"required"`
}

type LinkWalletUserByBVNRequest struct {
	CustomerID    string `json:"customer_id" binding:"required"`
	AccountUserID string `json:"account_user_id,omitempty"`
	BVN           string `json:"bvn" binding:"required"`
}

type LinkWalletUserByBVNResponse struct {
	LinkedUsers int64 `json:"linked_users"`
}
