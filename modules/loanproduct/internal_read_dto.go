package loanproduct

type GetLoanApplicationsForCBAResponse struct {
	Count        int                          `json:"count"`
	Applications []CBAListLoanApplicationItem `json:"applications"`
}

type GetEmbryoLoanApplicationsForCBAResponse struct {
	Count        int                            `json:"count"`
	Page         int                            `json:"page"`
	Limit        int                            `json:"limit"`
	Total        int64                          `json:"total"`
	Applications []CBAEmbryoLoanApplicationItem `json:"applications"`
}

type CBAEmbryoLoanApplicationItem struct {
	ApplicationRef string `json:"application_ref"`
	MobileUserID   string `json:"mobile_user_id"`
	Name           string `json:"name"`
	Gender         string `json:"gender"`
	PhoneNumber    string `json:"phone_number"`
	LoanStatus     string `json:"loan_status"`
	CustomerStatus string `json:"customer_status"`
}

type CBABVNRecordItem struct {
	ApplicationRef string              `json:"application_ref"`
	BVNRecord      CBABVNRecordReadDTO `json:"bvn_record"`
}

type GetLoanApplicationBVNRecordForCBAResponse struct {
	Record CBABVNRecordReadDTO `json:"customer_record"`
}

type CBAListLoanApplicationItem struct {
	ApplicationRef string                    `json:"application_ref"`
	Loan           CBALoanApplicationReadDTO `json:"loan"`
}

type GetLoanApplicationForCBAResponse struct {
	Application CBAListLoanApplicationItem `json:"application"`
}

type CBALoanApplicationReadDTO struct {
	ApplicationRef    string  `json:"application_ref"`
	MobileUserID      string  `json:"mobile_user_id"`
	CoreCustomerID    *string `json:"core_customer_id,omitempty"`
	Name              string  `json:"name"`
	PhoneNumber       string  `json:"phone_number"`
	LoanProductType   string  `json:"loan_product_type"`
	BusinessStartDate string  `json:"business_start_date"`
	BusinessAddress   string  `json:"business_address"`
	BusinessValue     int64   `json:"business_value"`
	BusinessType      string  `json:"business_type"`
	RequestedAmount   int64   `json:"requested_amount"`
	LoanStatus        string  `json:"loan_status"`
	Tenure            string  `json:"tenure"`
	TenureValue       int     `json:"tenure_value"`
}

type CBABVNRecordReadDTO struct {
	ApplicationRef         string  `json:"application_ref"`
	BVN                    string  `json:"bvn"`
	FirstName              string  `json:"first_name"`
	MiddleName             string  `json:"middle_name"`
	LastName               string  `json:"last_name"`
	Gender                 string  `json:"gender"`
	Nationality            string  `json:"nationality"`
	StateOfOrigin          string  `json:"state_of_origin"`
	DateOfBirth            string  `json:"date_of_birth"`
	EmailAddress           string  `json:"email_address"`
	MobilePhone            string  `json:"mobile_phone"`
	AlternativeMobilePhone *string `json:"alternative_mobile_phone,omitempty"`
	BankName               string  `json:"bank_name"`
	FullHomeAddress        string  `json:"full_home_address"`
	PassportOnBVN          string  `json:"passport_on_bvn"`
	City                   *string `json:"city,omitempty"`
	Landmark               *string `json:"landmark,omitempty"`
}

type LoanApplicationBVNRecordQuery struct {
	UserID string `form:"user_id"`
}

type LoanApplicationsForCBAQuery struct {
	UserID string `form:"user_id" binding:"required"`
}

type EmbryoLoanApplicationsForCBAQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}
