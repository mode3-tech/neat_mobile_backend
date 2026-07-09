package autorepayment

import "time"

type AutoRepaymentAttemptStatus string

const (
	AutoRepaymentAttemptStatusPending AutoRepaymentAttemptStatus = "pending"
	AutoRepaymentAttemptStatusSuccess AutoRepaymentAttemptStatus = "success"
	AutoRepaymentAttemptStatusFailed  AutoRepaymentAttemptStatus = "failed"
	AutoRepaymentAttemptStatusSkipped AutoRepaymentAttemptStatus = "skipped"
)

type DueRepaymentRow struct {
	RepaymentID    int64  `json:"repayment_id"`
	LoanID         int64  `json:"loan_id"`
	Amount         int64  `json:"amount"`
	MobileUserID   string `json:"mobile_user_id"`
	CoreCustomerID int64  `json:"core_customer_id"`
}

type BankResponse struct {
	Status bool   `json:"status"`
	Banks  []Bank `json:"banks"`
}

type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type BankDetailsResponse struct {
	Status  bool        `json:"status"`
	Account BankDetails `json:"account"`
}

type BankDetails struct {
	BankCode      string `json:"bankCode"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
}

type ProvidusCustomerDetailsResponse struct {
	Status   bool             `json:"status"`
	Customer ProvidusCustomer `json:"customer"`
}

type ProvidusCustomer struct {
	ID               string            `json:"id"`
	BVN              string            `json:"bvn"`
	FirstName        string            `json:"firstName"`
	LastName         string            `json:"lastName"`
	BVNLastName      string            `json:"bvnLastName"`
	BVNFirstName     string            `json:"bvnFirstName"`
	NameMatch        bool              `json:"nameMatch"`
	Email            string            `json:"email"`
	DateOfBirth      string            `json:"dateOfBirth"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	WalletID         string            `json:"walletId"`
	Metadata         map[string]string `json:"metadata"`
	Tier             string            `json:"tier"`
	DeletedAt        *time.Time        `json:"deletedAt"`
	AccountNumber    string            `json:"accountNumber"`
	BookedBalance    float64           `json:"bookedBalance"`
	AvailableBalance float64           `json:"availableBalance"`
}
