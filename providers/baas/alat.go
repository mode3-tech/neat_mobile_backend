package baas

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const (
	WemaBankCode = "000017"
	WemaBankName = "WEMA BANK"
	// WemaNameEnquiryBankCode is the bank code ALAT's AccountNameEnquiry takes
	// for a Wema account (the source wallet check before a transfer). It
	// differs from WemaBankCode, which is what we store on wallets.
	WemaNameEnquiryBankCode = "035"
)

// ALAT gateway API path prefixes. ALAT_BASE_URL is the gateway root
// (e.g. https://apiplayground.alat.ng); each product lives under its own prefix.
const (
	alatAccountCreationAPI = "/account-creation"      // Wallet Creation API - BVN
	alatFaceAPI            = "/create-account-face"   // Partnership Account - Face Biometric Authentication
	alatDebitWalletAPI     = "/debit-wallet"          // Debit Wallet API
	alatCreditWalletAPI    = "/credit-wallet"         // Credit Wallet API (wallet payout)
	alatAccountMgmtAPI     = "/ws-acct-mgt"           // Account Management API
	alatAccountUpgradeAPI  = "/account-upgrade"       // Account Upgrade API (tier 2 / tier 3)
	alatStatementAPI       = "/get-statement-service" // Get Statement API
	alatStaticWalletAPI    = "/staticwalletservice"   // Static Wallet Service (bulk static wallets)
	alatAirtimeDataAPI     = "/airtime-data"          // Airtime and Data API
	alatBillsPaymentAPI    = "/bills-payment"         // Bills Payment API
)

// apiURL joins the gateway root, a product prefix (alat*API), and an endpoint path.
func (a *Alat) apiURL(apiPrefix, path string) string {
	return a.BaseURL + apiPrefix + path
}

// Alat handles API interactions with Wema Bank's ALAT BaaS platform.
type Alat struct {
	BaseURL         string
	APIKey          string
	SubscriptionKey string
	Client          *http.Client
}

// NewAlat creates a new Wema ALAT BaaS client.
// baseURL must be the gateway root. A legacy value ending in a product prefix
// (e.g. ".../account-creation") is trimmed back to the root, so old env
// configs keep working.
func NewAlat(baseURL, apiKey, subscriptionKey, channelID string) *Alat {
	root := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, prefix := range []string{
		alatAccountCreationAPI, alatFaceAPI, alatDebitWalletAPI, alatCreditWalletAPI,
		alatAccountMgmtAPI, alatAccountUpgradeAPI, alatStatementAPI, alatStaticWalletAPI,
		alatAirtimeDataAPI, alatBillsPaymentAPI,
	} {
		root = strings.TrimSuffix(root, prefix)
	}
	return &Alat{
		BaseURL:         root,
		APIKey:          strings.TrimSpace(apiKey),
		SubscriptionKey: strings.TrimSpace(subscriptionKey),
		Client:          &http.Client{Timeout: 15 * time.Second},
	}
}

// Puts account generation for the customer in queue and returns a pending state immediately
func (a *Alat) CreateTier2AlatAccount(ctx context.Context, req AlatAccountCreationRequest) (*AlatAccountCreationResponse, error) {
	url := a.apiURL(alatFaceAPI, "/api/partnership/tier2-partnershipaccount-withoutOtp-v2")
	return alatDo[AlatAccountCreationResponse](ctx, a, http.MethodPost, url, req)
}

// Get Account Details with Phone number
func (a *Alat) GetAccountDetailsWithPhoneNumber(ctx context.Context, phoneNumber string) (*AlatAccountDetailsResponse, error) {
	url := a.apiURL(alatFaceAPI, fmt.Sprintf("/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", phoneNumber))
	return alatDo[AlatAccountDetailsResponse](ctx, a, http.MethodGet, url, nil)
}

// Puts tier 1 account generation (BVN, no OTP) in queue and returns a pending state immediately
func (a *Alat) CreateTier1AlatAccountWithBVN(ctx context.Context, req AlatTier1BVNRequest) (*AlatAccountCreationResponse, error) {
	url := a.apiURL(alatFaceAPI, "/api/partnership/tier1-bvn-withoutOtp-v2")
	return alatDo[AlatAccountCreationResponse](ctx, a, http.MethodPost, url, req)
}

// Puts tier 1 account generation (NIN, no OTP) in queue and returns a pending state immediately
func (a *Alat) CreateTier1AlatAccountWithNIN(ctx context.Context, req AlatTier1NINRequest) (*AlatAccountCreationResponse, error) {
	url := a.apiURL(alatFaceAPI, "/api/partnership/tier1-nin-withoutOtp-v2")
	return alatDo[AlatAccountCreationResponse](ctx, a, http.MethodPost, url, req)
}

// Wallet Creation (BVN) Step 1: Start BVN verification; ALAT sends the customer an OTP and returns a trackingId
func (a *Alat) PostPartnershipAccountCreationWithBvn(ctx context.Context, req AlatPartnershipRequestWithBvn) (*AlatAccountCreationResponse, error) {
	url := a.apiURL(alatAccountCreationAPI, "/api/CustomerAccount/PostPartnershipAccountCreationWithBvn")
	return alatDo[AlatAccountCreationResponse](ctx, a, http.MethodPost, url, req)
}

// Wallet Creation (BVN) Step 2: Validate the customer's OTP and queue account creation
func (a *Alat) ValidateBVNandEnqueueAccountCreation(ctx context.Context, req AlatValidateBvnRequest) (*AlatResponseModel, error) {
	url := a.apiURL(alatAccountCreationAPI, "/api/CustomerAccount/ValidateBVNandEnqueueAccountCreation")
	return alatDo[AlatResponseModel](ctx, a, http.MethodPost, url, req)
}

// Wallet Creation (BVN): Resend the step 1 OTP
func (a *Alat) ResendOtp(ctx context.Context, req AlatResendOtpRequest) (*AlatResponseModel, error) {
	url := a.apiURL(alatAccountCreationAPI, "/api/CustomerAccount/ResendOtpRequest/ResendOtp")
	return alatDo[AlatResponseModel](ctx, a, http.MethodPost, url, req)
}

// Wallet Creation (BVN) Step 3: Get account details by phone number for accounts created through this API
func (a *Alat) GetBVNAccountDetailsWithPhoneNumber(ctx context.Context, phoneNumber string) (*AlatAccountDetailsResponse, error) {
	url := a.apiURL(alatAccountCreationAPI, fmt.Sprintf("/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", phoneNumber))
	return alatDo[AlatAccountDetailsResponse](ctx, a, http.MethodGet, url, nil)
}

// Wallet Creation (BVN) Step 4: Lift or place the post-no-debit (PND) restriction on an account
func (a *Alat) PartnerDebitRestrictionManagement(ctx context.Context, req AlatPndRequest) (*AlatResponseModel, error) {
	url := a.apiURL(alatAccountCreationAPI, "/api/CustomerAccount/PartnerDebitRestrictionManagement")
	return alatDo[AlatResponseModel](ctx, a, http.MethodPost, url, req)
}

// Get Country, LGA, LCDA, State, House type dropdown list
func (a *Alat) GetAddressDropdownList(ctx context.Context) (*AlatAddressDropdownResponse, error) {
	url := a.apiURL(alatFaceAPI, "/api/CustomerInfo/GetDropDownList")
	return alatDo[AlatAddressDropdownResponse](ctx, a, http.MethodGet, url, nil)
}

// Get Wallet Detaiuls with Account Number
func (a *Alat) GetWalletDetailsWithAccountNumber(ctx context.Context, accountNumber string) (*AlatWalletDetailsResponse, error) {
	url := a.apiURL(alatAccountMgmtAPI, fmt.Sprintf("/api/AccountMaintenance/CustomerAccount/GetAccountV2/accountNumber/%s", accountNumber))
	return alatDo[AlatWalletDetailsResponse](ctx, a, http.MethodGet, url, nil)
}

// Get Transaction History (from, to and keyWord are optional filters)
func (a *Alat) GetTransactionHistory(ctx context.Context, req AlatTransactionHistoryRequest) (*AlatTransactionHistoryResponse, error) {
	url := a.apiURL(alatAccountMgmtAPI, "/api/AccountMaintenance/CustomerAccount/transhistoryV2")
	return alatDo[AlatTransactionHistoryResponse](ctx, a, http.MethodPost, url, req)
}

// Debit Wallet Step 1: Validate a wallet managed by our channel (the transfer source)
func (a *Alat) WalletNameEnquiry(ctx context.Context, accountNumber string) (*AlatWalletNameEnquiryResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, fmt.Sprintf("/api/Shared/AccountNameEnquiry/Wallet/%s", accountNumber))
	return alatDo[AlatWalletNameEnquiryResponse](ctx, a, http.MethodGet, url, nil)
}

// Debit Wallet Step 2: Get all banks and their bank codes
func (a *Alat) GetAllBanks(ctx context.Context) (*AlatBanksResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, "/api/Shared/GetAllBanks")
	return alatDo[AlatBanksResponse](ctx, a, http.MethodGet, url, nil)
}

// Debit Wallet Step 3: Validate a destination account with its bank code
func (a *Alat) AccountNameEnquiry(ctx context.Context, bankCode, accountNumber string) (*AlatAccountNameEnquiryResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, fmt.Sprintf("/api/Shared/AccountNameEnquiry/%s/%s", bankCode, accountNumber))
	return alatDo[AlatAccountNameEnquiryResponse](ctx, a, http.MethodGet, url, nil)
}

// Debit Wallet Step 4: Get the cost of interbank (NIP) transfers across amount bands
func (a *Alat) GetNIPCharges(ctx context.Context) (*AlatNIPChargesResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, "/api/Shared/GetNIPCharges")
	return alatDo[AlatNIPChargesResponse](ctx, a, http.MethodGet, url, nil)
}

// Debit Wallet Step 5: Debit a wallet and credit a destination account (intra or inter bank).
// Asynchronous: ALAT replies pending and confirms via the Authentication Callback.
func (a *Alat) ProcessClientTransfer(ctx context.Context, req AlatClientTransferRequest) (*AlatClientTransferResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, "/api/Shared/ProcessClientTransfer")
	return alatDo[AlatClientTransferResponse](ctx, a, http.MethodPost, url, req)
}

// Debit Wallet Step 6: Confirm a transfer's status by our transaction reference
func (a *Alat) ConfirmClientTransferStatus(ctx context.Context, clientTransactionReference string) (*AlatTransferStatusResponse, error) {
	url := a.apiURL(alatDebitWalletAPI, fmt.Sprintf("/api/IntraBankTransfer/ConfirmClientTransferStatus/%s", clientTransactionReference))
	return alatDo[AlatTransferStatusResponse](ctx, a, http.MethodGet, url, nil)
}

// Credit Wallet Step 1: Validate the destination wallet before a payout
func (a *Alat) CreditWalletNameEnquiry(ctx context.Context, accountNumber string) (*AlatWalletNameEnquiryResponse, error) {
	url := a.apiURL(alatCreditWalletAPI, fmt.Sprintf("/api/Shared/AccountNameEnquiry/Wallet/%s", accountNumber))
	return alatDo[AlatWalletNameEnquiryResponse](ctx, a, http.MethodGet, url, nil)
}

// Credit Wallet Step 2: Fund (pay out to) a wallet from our account.
// Asynchronous: ALAT replies pending and confirms via the Authentication Callback.
func (a *Alat) FundWallet(ctx context.Context, req AlatFundWalletRequest) (*AlatClientTransferResponse, error) {
	url := a.apiURL(alatCreditWalletAPI, "/api/IntraBankTransfer/FundWallet")
	return alatDo[AlatClientTransferResponse](ctx, a, http.MethodPost, url, req)
}

// Credit Wallet Step 3: Confirm a payout's status by our transaction reference
func (a *Alat) ConfirmFundWalletStatus(ctx context.Context, clientTransactionReference string) (*AlatTransferStatusResponse, error) {
	url := a.apiURL(alatCreditWalletAPI, fmt.Sprintf("/api/IntraBankTransfer/ConfirmClientTransferStatus/%s", clientTransactionReference))
	return alatDo[AlatTransferStatusResponse](ctx, a, http.MethodGet, url, nil)
}

// Account Upgrade: Upgrade a partnership account to tier 2 with NIN, BVN and a live face image
func (a *Alat) UpgradeToTier2(ctx context.Context, req AlatTier2UpgradeRequest) (*AlatAccountUpgradeResponse, error) {
	url := a.apiURL(alatAccountUpgradeAPI, "/api/partnership/partner-account-upgrade-tier2")
	return alatDo[AlatAccountUpgradeResponse](ctx, a, http.MethodPost, url, req)
}

// Account Upgrade: Upgrade a partnership account to tier 3 with a residential address
func (a *Alat) UpgradeToTier3(ctx context.Context, req AlatTier3UpgradeRequest) (*AlatAccountUpgradeResponse, error) {
	url := a.apiURL(alatAccountUpgradeAPI, "/api/partnership/partner-account-upgrade-tier3")
	return alatDo[AlatAccountUpgradeResponse](ctx, a, http.MethodPost, url, req)
}

// Account Upgrade: Get an account's tier, status, restriction and address verification status
func (a *Alat) GetKYCStatus(ctx context.Context, accountNumber string) (*AlatKYCStatusResponse, error) {
	url := a.apiURL(alatAccountUpgradeAPI, "/api/partnership/partner-account-kyc-status?accountNumber="+neturl.QueryEscape(accountNumber))
	return alatDo[AlatKYCStatusResponse](ctx, a, http.MethodGet, url, nil)
}

// Static Wallet: Aggregate figures (created, active, inactive, awaiting disbursement) across all our bulk accounts
func (a *Alat) GetBulkAccountsSummary(ctx context.Context) (*AlatBulkAccountsSummaryResponse, error) {
	url := a.apiURL(alatStaticWalletAPI, "/api/v1/bulk-accounts/summary")
	return alatDo[AlatBulkAccountsSummaryResponse](ctx, a, http.MethodGet, url, nil)
}

// Static Wallet: Balance of one account at the time of the request
func (a *Alat) GetBulkAccountBalance(ctx context.Context, req AlatBalanceEnquiryRequest) (*AlatBalanceEnquiryResponse, error) {
	url := a.apiURL(alatStaticWalletAPI, "/api/v1/bulk-accounts/balance")
	return alatDo[AlatBalanceEnquiryResponse](ctx, a, http.MethodPost, url, req)
}

// Get Statement Step 1: Start a statement request for an account over a date range; returns a referenceId
func (a *Alat) InitiateCustomerStatement(ctx context.Context, req AlatInitiateStatementRequest) (*AlatInitiateStatementResponse, error) {
	url := a.apiURL(alatStatementAPI, "/api/AccountMaintenance/InitiateGetCustomerStatement")
	return alatDo[AlatInitiateStatementResponse](ctx, a, http.MethodPost, url, req)
}

// Get Statement Step 2: Fetch the statement's transactions by the referenceId from step 1
func (a *Alat) GetCustomerTransactions(ctx context.Context, req AlatCustomerTransactionsRequest) (*AlatCustomerTransactionsResponse, error) {
	url := a.apiURL(alatStatementAPI, "/api/AccountMaintenance/GetCustomerTransactions")
	return alatDo[AlatCustomerTransactionsResponse](ctx, a, http.MethodPost, url, req)
}
