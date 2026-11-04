package baas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	WemaBankCode = "000017"
	WemaBankName = "WEMA BANK"
)

// Alat handles API interactions with Wema Bank's ALAT BaaS platform.
type Alat struct {
	BaseURL         string
	APIKey          string
	SubscriptionKey string
	Client          *http.Client
}

// NewAlat creates a new Wema ALAT BaaS client.
func NewAlat(baseURL, apiKey, subscriptionKey string) *Alat {
	return &Alat{
		BaseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:          strings.TrimSpace(apiKey),
		SubscriptionKey: strings.TrimSpace(subscriptionKey),
		Client:          &http.Client{Timeout: 60 * time.Second},
	}
}

// AlatPartnershipRequestWithBvn represents Step 1 payload for BVN wallet creation.
type AlatPartnershipRequestWithBvn struct {
	PhoneNumber string `json:"phoneNumber"`
	Email       string `json:"email"`
	BVN         string `json:"bvn"`
}

// AlatValidateBvnRequest represents Step 2 payload for validating OTP and enqueuing account creation.
type AlatValidateBvnRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTP         string `json:"otp"`
	TrackingID  string `json:"trackingId"`
}

// AlatResendOtpRequest represents payload for resending OTP during wallet creation.
type AlatResendOtpRequest struct {
	PhoneNumber string `json:"phoneNumber,omitempty"`
	TrackingID  string `json:"trackingId,omitempty"`
}

// AlatPndRequest represents Step 4 payload for managing debit restrictions (LiftPnd or PlacePnd).
type AlatPndRequest struct {
	PndType       string `json:"pndType"` // e.g. "LiftPnd", "PlacePnd"
	AccountNumber string `json:"accountNumber"`
}

// AlatStep1Data holds data returned by Step 1, including the tracking ID needed for Step 2.
type AlatStep1Data struct {
	TrackingID                string  `json:"trackingId"`
	AccountGenerationStatus   *string `json:"accountGenerationStatus"`
	AddressVerificationStatus *string `json:"addressVerificationStatus"`
}

// AlatResponseModel represents the generic response envelope returned by ALAT.
type AlatResponseModel struct {
	Message    string         `json:"message"`
	Status     bool           `json:"status"`
	Code       any            `json:"code"`
	StatusCode any            `json:"statusCode"`
	Errors     []string       `json:"errors"`
	Data       *AlatStep1Data `json:"data,omitempty"`
}

func (r *AlatResponseModel) GetCode() string {
	if r == nil || r.Code == nil {
		return ""
	}
	return fmt.Sprint(r.Code)
}

func (r *AlatResponseModel) GetStatusCode() string {
	if r == nil || r.StatusCode == nil {
		return ""
	}
	return fmt.Sprint(r.StatusCode)
}

// AlatAccountDetails contains the bank account info returned in Step 3.
type AlatAccountDetails struct {
	AccountNumber string `json:"accountNumber"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Email         string `json:"email"`
	PhoneNumber   string `json:"phoneNumber"`
}

// AlatAccountDetailsResponse represents the response envelope for Step 3.
type AlatAccountDetailsResponse struct {
	Message    string              `json:"message"`
	Status     bool                `json:"status"`
	Code       any                 `json:"code"`
	StatusCode any                 `json:"statusCode"`
	Errors     []string            `json:"errors"`
	Data       *AlatAccountDetails `json:"data"`
}

func (r *AlatAccountDetailsResponse) GetCode() string {
	if r == nil || r.Code == nil {
		return ""
	}
	return fmt.Sprint(r.Code)
}

func (r *AlatAccountDetailsResponse) GetStatusCode() string {
	if r == nil || r.StatusCode == nil {
		return ""
	}
	return fmt.Sprint(r.StatusCode)
}

// Step 1: PostPartnershipAccountCreationWithBvn sends customer BVN, phone, and email to ALAT,
// which initiates the verification flow and triggers an OTP to the customer.
func (a *Alat) PostPartnershipAccountCreationWithBvn(ctx context.Context, req AlatPartnershipRequestWithBvn) (*AlatResponseModel, error) {
	endpoint := a.BaseURL + "/api/CustomerAccount/PostPartnershipAccountCreationWithBvn"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, &appErr.AlatError{
			StatusCode: resp.GetStatusCode(),
			Code:       resp.GetCode(),
			Message:    resp.Message,
			Errors:     resp.Errors,
		}
	}
	return &resp, nil
}

// Step 2: ValidateBVNandEnqueueAccountCreation validates customer OTP and trackingId to enqueue account creation.
func (a *Alat) ValidateBVNandEnqueueAccountCreation(ctx context.Context, req AlatValidateBvnRequest) (*AlatResponseModel, error) {
	endpoint := a.BaseURL + "/api/CustomerAccount/ValidateBVNandEnqueueAccountCreation"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, &appErr.AlatError{
			StatusCode: resp.GetStatusCode(),
			Code:       resp.GetCode(),
			Message:    resp.Message,
			Errors:     resp.Errors,
		}
	}
	return &resp, nil
}

// Step 3: GetPartnershipAccountDetails fetches the created account details using the customer's phone number.
func (a *Alat) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetailsResponse, error) {
	endpoint := fmt.Sprintf("%s/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", a.BaseURL, url.QueryEscape(phoneNumber))
	var resp AlatAccountDetailsResponse
	if err := a.doRequest(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, &appErr.AlatError{
			StatusCode: resp.GetStatusCode(),
			Code:       resp.GetCode(),
			Message:    resp.Message,
			Errors:     resp.Errors,
		}
	}
	return &resp, nil
}

// Step 4: PartnerDebitRestrictionManagement removes or applies PND on an account (e.g. LiftPnd).
func (a *Alat) PartnerDebitRestrictionManagement(ctx context.Context, req AlatPndRequest) (*AlatResponseModel, error) {
	endpoint := a.BaseURL + "/api/CustomerAccount/PartnerDebitRestrictionManagement"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, &appErr.AlatError{
			StatusCode: resp.GetStatusCode(),
			Code:       resp.GetCode(),
			Message:    resp.Message,
			Errors:     resp.Errors,
		}
	}
	return &resp, nil
}

// ResendOtp resends the verification OTP for account creation.
func (a *Alat) ResendOtp(ctx context.Context, req AlatResendOtpRequest) (*AlatResponseModel, error) {
	endpoint := a.BaseURL + "/api/CustomerAccount/ResendOtpRequest/ResendOtp"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, &appErr.AlatError{
			StatusCode: resp.GetStatusCode(),
			Code:       resp.GetCode(),
			Message:    resp.Message,
			Errors:     resp.Errors,
		}
	}
	return &resp, nil
}

// LookupWalletByCustomerID looks up an existing Wema ALAT account by phone number to satisfy auth.WalletService.
func (a *Alat) LookupWalletByCustomerID(ctx context.Context, phoneOrID string) (*auth.WalletResponse, bool, error) {
	resp, err := a.GetPartnershipAccountDetails(ctx, phoneOrID)
	if err != nil {
		var alatErr *appErr.AlatError
		if errors.As(err, &alatErr) {
			log.Printf("alat lookup wallet error: %v", alatErr)
			return nil, false, nil
		}
		return nil, false, err
	}
	if resp == nil || resp.Data == nil || strings.TrimSpace(resp.Data.AccountNumber) == "" {
		return nil, false, nil
	}

	fullName := strings.TrimSpace(resp.Data.FirstName + " " + resp.Data.LastName)
	return &auth.WalletResponse{
		Customer: &auth.WalletCustomer{
			ID:       phoneOrID,
			Currency: "NGN",
		},
		Wallet: &auth.WalletInfo{
			AccountNumber: resp.Data.AccountNumber,
			AccountName:   fullName,
			BankCode:      WemaBankCode,
			BankName:      WemaBankName,
			WalletId:      resp.Data.AccountNumber,
		},
	}, true, nil
}

// GenerateWallet initiates or completes account creation depending on whether the account already exists.
func (a *Alat) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	if walletInfo == nil {
		return nil, errors.New("wallet payload is required")
	}

	// First check if account details already exist
	existing, found, err := a.LookupWalletByCustomerID(ctx, walletInfo.PhoneNumber)
	if err == nil && found {
		return existing, nil
	}

	// Trigger Step 1 account creation with BVN
	_, err = a.PostPartnershipAccountCreationWithBvn(ctx, AlatPartnershipRequestWithBvn{
		PhoneNumber: walletInfo.PhoneNumber,
		Email:       walletInfo.Email,
		BVN:         walletInfo.BVN,
	})
	if err != nil {
		return nil, fmt.Errorf("alat wallet initiation failed: %w", err)
	}

	return nil, errors.New("alat wallet initiation successful: otp required to complete account creation")
}

func (a *Alat) doRequest(ctx context.Context, method, endpoint string, body any, target any) error {
	if a.BaseURL == "" || a.APIKey == "" {
		return errors.New("wema alat service is not configured (missing ALAT_BASE_URL or ALAT_API_KEY)")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("alat marshal request failed: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return fmt.Errorf("alat create request failed: %w", err)
	}

	if a.APIKey != "" {
		req.Header.Set("x-api-key", a.APIKey)
	}
	if a.SubscriptionKey != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", a.SubscriptionKey)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		log.Printf("alat http request failed [%s %s]: %v", method, endpoint, err)
		return fmt.Errorf("alat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("alat read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var alatErr appErr.AlatError
		if json.Unmarshal(respBytes, &alatErr) == nil && (alatErr.Message != "" || alatErr.Code != "" || len(alatErr.Errors) > 0) {
			return &alatErr
		}
		return fmt.Errorf("alat request returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if target != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, target); err != nil {
			log.Printf("alat decode response failed body=%s err=%v", string(respBytes), err)
			return fmt.Errorf("alat decode response failed: %w", err)
		}
	}

	return nil
}

