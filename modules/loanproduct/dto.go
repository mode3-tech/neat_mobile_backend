package loanproduct

type LoanRequest struct {
	LoanProductType   LoanType `json:"loan_product_type" binding:"required"`
	BusinessAddress   string   `json:"business_address" binding:"required"`
	BusinessStartDate string   `json:"business_start_date" binding:"required,datetime=2006-01"`
	BusinessValue     string   `json:"business_value" binding:"required"`
	BusinessType      string   `json:"business_type" binding:"required"`
	LoanAmount        string   `json:"loan_amount" binding:"required"`
}

type LoanSummaryResponse struct {
	BusinessValue       int64         `json:"business_value"`
	BusinessAgeYears    int           `json:"business_age_years"`
	LoanAmount          int64         `json:"loan_amount"`
	InterestRatePercent float64       `json:"interest_rate_percent"`
	InterestAmount      float64       `json:"interest_amount"`
	TotalRepayment      float64       `json:"total_repayment"`
	PeriodicRepayment   float64       `json:"periodic_repayment"`
	LoanTermValue       int           `json:"loan_term_value"`
	RepaymentFrequency  LoanFrequency `json:"repayment_frequency"`
	IsEstimate          bool          `json:"is_estimate"`
}

type ApplyForLoanResponse struct {
	Message        string              `json:"message"`
	ApplicationRef string              `json:"application_ref"`
	LoanStatus     LoanStatus          `json:"loan_status"`
	Summary        LoanSummaryResponse `json:"summary"`
}
