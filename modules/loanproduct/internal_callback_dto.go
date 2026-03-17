package loanproduct

type UpdateLoanApplicationStatusRequest struct {
	EventID    string `json:"event_id" binding:"required"`
	Status     string `json:"status" binding:"required"`
	CoreLoanID string `json:"core_loan_id"`
}
