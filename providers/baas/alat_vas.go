package baas

import (
	"context"
	"net/http"
	"strings"

	appErr "neat_mobile_app_backend/internal/errors"
)

type AlatClientAirtimeRequest struct {
	TransactionReference string  `json:"transactionReference"`
	AccountNumber        string  `json:"accountNumber"`
	Network              string  `json:"network"`
	PhoneNumber          string  `json:"phoneNumber"`
	Amount               float64 `json:"amount"`
	SecurityInfo         string  `json:"securityInfo"`
}

type AlatClientDataRequest struct {
	TransactionReference string  `json:"transactionReference"`
	AccountNumber        string  `json:"accountNumber"`
	PhoneNumber          string  `json:"phoneNumber"`
	PackageCode          int     `json:"packageCode"`
	Amount               float64 `json:"amount"`
	Network              string  `json:"network"`
	SecurityInfo         string  `json:"securityInfo"`
}

type AlatClientBillRequest struct {
	ClientID             string  `json:"clientId,omitempty"`
	CustomerAccount      string  `json:"customerAccount"`
	Amount               float64 `json:"amount"`
	Charge               float64 `json:"charge"`
	TransactionReference string  `json:"transactionReference"`
	PackageID            int     `json:"packageId"`
	CustomerIdentifier   string  `json:"customerIdentifier,omitempty"`
	CustomerEmail        string  `json:"customerEmail,omitempty"`
	CustomerPhoneNumber  string  `json:"customerPhoneNumber,omitempty"`
	CustomerName         string  `json:"customerName,omitempty"`
	SecurityInfo         string  `json:"securityInfo"`
}

type AlatClientVASTransaction struct {
	Status                       string `json:"status"`
	Message                      string `json:"message"`
	Narration                    string `json:"narration"`
	TransactionReference         string `json:"transactionReference"`
	PlatformTransactionReference string `json:"platformTransactionReference"`
	TransactionStan              string `json:"transactionStan"`
	Value                        string `json:"value"`
	OriginalTxnTransactionDate   string `json:"orinalTxnTransactionDate"`
}

type AlatClientVASResponse struct {
	Result        *AlatClientVASTransaction `json:"result"`
	ErrorMessage  string                    `json:"errorMessage"`
	ErrorMessages []string                  `json:"errorMessages"`
	HasError      bool                      `json:"hasError"`
	TimeGenerated string                    `json:"timeGenerated"`
}

type AlatBillPackage struct {
	ID               int     `json:"id"`
	BillerID         int     `json:"billerId"`
	Name             string  `json:"name"`
	IsAmountEditable bool    `json:"isAmountEditable"`
	Amount           float64 `json:"amount"`
	MinAmount        float64 `json:"minAmount"`
	MaxAmount        float64 `json:"maxAmount"`
}

type AlatBiller struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	Identifier         string            `json:"identifier"`
	ShortCode          string            `json:"shortCode"`
	IsAcquired         bool              `json:"isAquired"`
	RequiredValidation bool              `json:"requiredValidation"`
	Charge             float64           `json:"charge"`
	Flow               int               `json:"flow"`
	Packages           []AlatBillPackage `json:"packages"`
}

type AlatBillCategory struct {
	ID      int          `json:"id"`
	Name    string       `json:"name"`
	Billers []AlatBiller `json:"billers"`
}

type AlatBillsResponse struct {
	Result        []AlatBillCategory `json:"result"`
	ErrorMessage  string             `json:"errorMessage"`
	ErrorMessages []string           `json:"errorMessages"`
	HasError      bool               `json:"hasError"`
	TimeGenerated string             `json:"timeGenerated"`
}

type AlatBillValidationRequest struct {
	Identifier string `json:"identifier"`
	PackageID  int    `json:"packageId"`
}

type AlatBillValidation struct {
	IsValidated    bool    `json:"isValidated"`
	CustomerName   string  `json:"customerName"`
	Message        string  `json:"message"`
	ValidationInfo string  `json:"validationInfo"`
	CreditLimit    float64 `json:"creditLimit"`
}

type AlatBillValidationResponse struct {
	Result        *AlatBillValidation `json:"result"`
	ErrorMessage  string              `json:"errorMessage"`
	ErrorMessages []string            `json:"errorMessages"`
	HasError      bool                `json:"hasError"`
	TimeGenerated string              `json:"timeGenerated"`
}

type AlatDataPackage struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	DataPlan       string  `json:"dataPlan"`
	ValidityPeriod string  `json:"validity_Period"`
	Description    string  `json:"description"`
}

type AlatDataPlan struct {
	ID              int               `json:"id"`
	NetworkProvider string            `json:"networkProvider"`
	DataPackages    []AlatDataPackage `json:"dataPackages"`
}

type AlatDataPlansResponse struct {
	Result        []AlatDataPlan `json:"result"`
	ErrorMessage  string         `json:"errorMessage"`
	ErrorMessages []string       `json:"errorMessages"`
	HasError      bool           `json:"hasError"`
	TimeGenerated string         `json:"timeGenerated"`
}

// GetAllBills fetches ALAT's bill categories, billers, and packages.
func (a *Alat) GetAllBills(ctx context.Context) (*AlatBillsResponse, error) {
	url := a.apiURL(alatBillsPaymentAPI, "/api/BillsPayment/GetAllBills")
	return alatDo[AlatBillsResponse](ctx, a, http.MethodGet, url, nil)
}

// ValidateBill validates a customer identifier against an ALAT bill package.
func (a *Alat) ValidateBill(ctx context.Context, req AlatBillValidationRequest) (*AlatBillValidationResponse, error) {
	url := a.apiURL(alatBillsPaymentAPI, "/api/BillsPayment/ValidateCustomer")
	return alatDo[AlatBillValidationResponse](ctx, a, http.MethodPost, url, req)
}

// PayClientBill pays a bill directly from a customer's ALAT account.
func (a *Alat) PayClientBill(ctx context.Context, req AlatClientBillRequest) (*AlatClientVASResponse, error) {
	url := a.apiURL(alatBillsPaymentAPI, "/api/Shared/PayBill")
	return alatDo[AlatClientVASResponse](ctx, a, http.MethodPost, url, req)
}

// PurchaseClientAirtime purchases airtime directly from a customer's ALAT account.
func (a *Alat) PurchaseClientAirtime(ctx context.Context, req AlatClientAirtimeRequest) (*AlatClientVASResponse, error) {
	url := a.apiURL(alatAirtimeDataAPI, "/api/Airtime/Client/PurchaseAirtime")
	return alatDo[AlatClientVASResponse](ctx, a, http.MethodPost, url, req)
}

// PurchaseClientData purchases a data package directly from a customer's ALAT account.
func (a *Alat) PurchaseClientData(ctx context.Context, req AlatClientDataRequest) (*AlatClientVASResponse, error) {
	url := a.apiURL(alatAirtimeDataAPI, "/api/Data/Client/PurchaseData")
	return alatDo[AlatClientVASResponse](ctx, a, http.MethodPost, url, req)
}

// GetDataPlans fetches ALAT's available data plans.
func (a *Alat) GetDataPlans(ctx context.Context) (*AlatDataPlansResponse, error) {
	url := a.apiURL(alatAirtimeDataAPI, "/api/Data/GetDataPlans")
	return alatDo[AlatDataPlansResponse](ctx, a, http.MethodGet, url, nil)
}

func AlatResultError(hasError bool, message string, messages []string) error {
	if !hasError {
		return nil
	}
	message = strings.TrimSpace(message)
	if message == "" && len(messages) > 0 {
		message = strings.TrimSpace(strings.Join(messages, "; "))
	}
	if message == "" {
		message = "ALAT reported a VAS error"
	}
	return &appErr.AlatError{Message: message, Errors: messages}
}
