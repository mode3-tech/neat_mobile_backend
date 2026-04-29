package reporting

import (
	"neat_mobile_app_backend/modules/transaction"
	"time"
)

type ListSignedUsersQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

type signedUserRow struct {
	ID                string     `gorm:"column:id"`
	FirstName         string     `gorm:"column:first_name"`
	LastName          string     `gorm:"column:last_name"`
	MiddleName        *string    `gorm:"column:middle_name"`
	Email             string     `gorm:"column:email"`
	Phone             string     `gorm:"column:phone"`
	BVN               string     `gorm:"column:bvn"`
	CoreCustomerID    *string    `gorm:"column:core_customer_id"`
	CustomerStatus    *string    `gorm:"column:customer_status"`
	Username          *string    `gorm:"column:username"`
	IsBVNVerified     bool       `gorm:"column:is_bvn_verified"`
	IsNINVerified     bool       `gorm:"column:is_nin_verified"`
	IsPhoneVerified   bool       `gorm:"column:is_phone_verified"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	BVNFirstName      *string    `gorm:"column:bvn_first_name"`
	BVNLastName       *string    `gorm:"column:bvn_last_name"`
	LoanStatus        *string    `gorm:"column:loan_status"`
	LastLoanAppliedAt *time.Time `gorm:"column:last_loan_applied_at"`
}

type SignedUserItem struct {
	MobileUserID   string             `json:"mobile_user_id"`
	FirstName      string             `json:"first_name"`
	LastName       string             `json:"last_name"`
	MiddleName     *string            `json:"middle_name,omitempty"`
	Email          string             `json:"email"`
	Phone          string             `json:"phone"`
	BVN            string             `json:"bvn"`
	CoreCustomerID *string            `json:"core_customer_id,omitempty"`
	CustomerStatus *string            `json:"customer_status,omitempty"`
	Username       *string            `json:"username,omitempty"`
	Verified       VerifiedFlags      `json:"verified"`
	LatestLoan     *LatestLoanSummary `json:"latest_loan,omitempty"`
	RegisteredAt   time.Time          `json:"registered_at"`
}

type VerifiedFlags struct {
	BVN   bool `json:"bvn"`
	NIN   bool `json:"nin"`
	Phone bool `json:"phone"`
}

type LatestLoanSummary struct {
	Status    string    `json:"status"`
	AppliedAt time.Time `json:"applied_at"`
}

type ListSignedUsersResponse struct {
	Users      []SignedUserItem `json:"users"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
}

type userTransactionRow struct {
	MobileUserID         string                        `gorm:"column:mobile_user_id"`
	Type                 transaction.TransactionType   `gorm:"column:type"`
	Amount               int64                         `gorm:"column:amount"`
	Charges              int64                         `gorm:"column:charges"`
	VAT                  int64                         `gorm:"column:vat"`
	BalanceBefore        int64                         `gorm:"column:balance_before"`
	BalanceAfter         int64                         `gorm:"column:balance_after"`
	TransactionReference string                        `gorm:"column:transaction_reference"`
	Narration            *string                       `gorm:"column:narration"`
	RecipientName        string                        `gorm:"column:recipient_name"`
	RecipientAccount     string                        `gorm:"column:recipient_account"`
	RecipientBank        string                        `gorm:"column:recipient_bank"`
	Status               transaction.TransactionStatus `gorm:"column:status"`
	CreatedAt            time.Time                     `gorm:"column:created_at"`
}

type UserTransactionQuery struct {
	MobileUserID string `form:"mobile_user_id" binding:"required"`
	Page         int    `json:"form:page"`
	Limit        int    `json:"form:limit"`
}

type UserTransaction struct {
	MobileUserID         string                        `json:"mobile_user_id"`
	Type                 transaction.TransactionType   `json:"type"`
	Amount               float64                       `json:"amount"`
	Charges              float64                       `json:"charges"`
	VAT                  float64                       `json:"vat"`
	BalanceBefore        float64                       `json:"balance_before"`
	BalanceAfter         float64                       `json:"balance_after"`
	TransactionReference string                        `json:"transaction_reference"`
	Narration            *string                       `json:"narration"`
	RecipientName        string                        `json:"recipient_name"`
	RecipientAccount     string                        `json:"recipient_account"`
	RecipientBank        string                        `json:"recipient_bank"`
	Status               transaction.TransactionStatus `json:"status"`
	CreatedAt            time.Time                     `json:"created_at"`
}

type UserTransactionResponse struct {
	Transactions []UserTransaction `json:"transactions"`
	Total        int64             `json:"total"`
	Page         int               `json:"page"`
	Limit        int               `json:"limit"`
	TotalPages   int               `json:"total_pages"`
}
