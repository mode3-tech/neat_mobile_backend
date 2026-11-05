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

	"github.com/google/uuid"
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
		Client:          &http.Client{Timeout: 15 * time.Second},
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

// AlatResidentialAddress represents customer address details for Tier 2 face account opening.
type AlatResidentialAddress struct {
	BuildingNumber string `json:"buildingNumber"`
	Street         string `json:"street"`
	City           string `json:"city"`
	State          string `json:"state"`
	Lga            string `json:"lga"`
	Landmark       string `json:"landmark"`
	Country        string `json:"country"`
	FullAddress    string `json:"fullAddress"`
}

// AlatTier2PartnershipRequestWithFace represents Step 1c payload for creating Tier 2 account with face biometrics.
type AlatTier2PartnershipRequestWithFace struct {
	BVN                string                  `json:"bvn"`
	NIN                string                  `json:"nin"`
	PhoneNumber        string                  `json:"phoneNumber"`
	EmailAddress       string                  `json:"emailAddress"`
	ResidentialAddress *AlatResidentialAddress `json:"residentialAddress,omitempty"`
	LiveImageOfFace    string                  `json:"liveImageOfFace"`
	CorrelationID      string                  `json:"correlationId"`
}

// AlatTier1BvnWithoutOtpRequest represents Step 1a payload for creating Tier 1 account with BVN without OTP.
type AlatTier1BvnWithoutOtpRequest struct {
	PhoneNumber   string `json:"phoneNumber"`
	Email         string `json:"email"`
	BVN           string `json:"bvn"`
	CorrelationID string `json:"correlationId"`
}

// AlatTier1NinWithoutOtpRequest represents Step 1b payload for creating Tier 1 account with NIN without OTP.
type AlatTier1NinWithoutOtpRequest struct {
	PhoneNumber   string `json:"phoneNumber"`
	Email         string `json:"email"`
	NIN           string `json:"nin"`
	CorrelationID string `json:"correlationId"`
}

// Dropdown models for Step 0 (GetAddressDropDownList)
type AlatCountry struct {
	ID          int64  `json:"id"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
}

type AlatState struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	FinacleCode string `json:"finacleCode"`
	Country     string `json:"country"`
}

type AlatLGA struct {
	LgaID   int32  `json:"lgaId"`
	StateID int32  `json:"stateId"`
	Name    string `json:"name"`
}

type AlatLCDA struct {
	LcdaID int32  `json:"lcdaId"`
	LgaID  int32  `json:"lgaId"`
	Name   string `json:"name"`
}

type AlatCity struct {
	ID      int64  `json:"id"`
	StateID int32  `json:"stateId"`
	Name    string `json:"name"`
}

type AlatHousingType struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
	Code  string `json:"code"`
}

type AlatCountryModel struct {
	CountryList  []AlatCountry     `json:"countryList"`
	StateList    []AlatState       `json:"stateList"`
	LgaList      []AlatLGA         `json:"lgaList"`
	LcdaList     []AlatLCDA        `json:"lcdaList"`
	CityList     []AlatCity        `json:"cityList"`
	HousingTypes []AlatHousingType `json:"housingTypes"`
}

type AlatDropDownListData struct {
	CountryModel AlatCountryModel `json:"countryModel"`
}

type AlatDropDownListResponse struct {
	Message    string                `json:"message"`
	Status     bool                  `json:"status"`
	Code       any                   `json:"code"`
	StatusCode any                   `json:"statusCode"`
	Errors     []string              `json:"errors"`
	Data       *AlatDropDownListData `json:"data,omitempty"`
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
		return &resp, envelopeError("PostPartnershipAccountCreationWithBvn", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
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
		return &resp, envelopeError("ValidateBVNandEnqueueAccountCreation", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// faceBaseURL returns the root URL with the /create-account-face prefix for biometric APIs.
func (a *Alat) faceBaseURL() string {
	root := strings.TrimRight(a.BaseURL, "/")
	root = strings.TrimSuffix(root, "/account-creation")
	root = strings.TrimSuffix(root, "/create-account-face")
	return root + "/create-account-face"
}

// Step 0: GetAddressDropDownList fetches country, state, LGA, LCDA, city, and housing types.
func (a *Alat) GetAddressDropDownList(ctx context.Context) (*AlatDropDownListResponse, error) {
	endpoint := a.faceBaseURL() + "/api/CustomerInfo/GetDropDownList"
	var resp AlatDropDownListResponse
	if err := a.doRequest(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Step 1a: PostTier1BvnWithoutOtp creates a Tier 1 account using BVN without SMS OTP.
func (a *Alat) PostTier1BvnWithoutOtp(ctx context.Context, req AlatTier1BvnWithoutOtpRequest) (*AlatResponseModel, error) {
	endpoint := a.faceBaseURL() + "/api/partnership/tier1-bvn-withoutOtp-v2"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, envelopeError("PostTier1BvnWithoutOtp", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// Step 1b: PostTier1NinWithoutOtp creates a Tier 1 account using NIN without SMS OTP.
func (a *Alat) PostTier1NinWithoutOtp(ctx context.Context, req AlatTier1NinWithoutOtpRequest) (*AlatResponseModel, error) {
	endpoint := a.faceBaseURL() + "/api/partnership/tier1-nin-withoutOtp-v2"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, envelopeError("PostTier1NinWithoutOtp", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// Step 1c: PostTier2PartnershipAccountWithFace creates a Tier 2 account with BVN, NIN, live face image, and residential address without SMS OTP.
func (a *Alat) PostTier2PartnershipAccountWithFace(ctx context.Context, req AlatTier2PartnershipRequestWithFace) (*AlatResponseModel, error) {
	endpoint := a.faceBaseURL() + "/api/partnership/tier2-partnershipaccount-withoutOtp-v2"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, envelopeError("PostTier2PartnershipAccountWithFace", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// Step 2 / Step 3: GetPartnershipAccountDetails fetches the created account details using the customer's phone number.
// Tries the standard endpoint first, and falls back to the face biometric endpoint if needed.
func (a *Alat) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetailsResponse, error) {
	endpoint := fmt.Sprintf("%s/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", a.BaseURL, url.QueryEscape(phoneNumber))
	var resp AlatAccountDetailsResponse
	err := a.doRequest(ctx, http.MethodGet, endpoint, nil, &resp)
	if err == nil && resp.Status && resp.Data != nil && strings.TrimSpace(resp.Data.AccountNumber) != "" {
		return &resp, nil
	}

	// Try face biometric endpoint
	faceEndpoint := fmt.Sprintf("%s/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", a.faceBaseURL(), url.QueryEscape(phoneNumber))
	var faceResp AlatAccountDetailsResponse
	faceErr := a.doRequest(ctx, http.MethodGet, faceEndpoint, nil, &faceResp)
	if faceErr == nil && faceResp.Status && faceResp.Data != nil && strings.TrimSpace(faceResp.Data.AccountNumber) != "" {
		return &faceResp, nil
	}
	log.Printf("[ALAT] level=INFO op=GetPartnershipAccountDetails no account on either endpoint phone=%s standard=(err=%v status=%t message=%q) face=(err=%v status=%t message=%q)",
		maskTail(phoneNumber), err, resp.Status, resp.Message, faceErr, faceResp.Status, faceResp.Message)

	if err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, envelopeError("GetPartnershipAccountDetails", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// GetPartnershipAccountDetailsWithFace fetches account details preferring the face biometric endpoint.
func (a *Alat) GetPartnershipAccountDetailsWithFace(ctx context.Context, phoneNumber string) (*AlatAccountDetailsResponse, error) {
	faceEndpoint := fmt.Sprintf("%s/api/CustomerAccount/GetPartnershipAccountDetails?phoneNumber=%s", a.faceBaseURL(), url.QueryEscape(phoneNumber))
	var resp AlatAccountDetailsResponse
	err := a.doRequest(ctx, http.MethodGet, faceEndpoint, nil, &resp)
	if err == nil && resp.Status && resp.Data != nil && strings.TrimSpace(resp.Data.AccountNumber) != "" {
		return &resp, nil
	}

	return a.GetPartnershipAccountDetails(ctx, phoneNumber)
}

// Step 4: PartnerDebitRestrictionManagement removes or applies PND on an account (e.g. LiftPnd).
func (a *Alat) PartnerDebitRestrictionManagement(ctx context.Context, req AlatPndRequest) (*AlatResponseModel, error) {
	endpoint := a.BaseURL + "/api/CustomerAccount/PartnerDebitRestrictionManagement"
	var resp AlatResponseModel
	if err := a.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return &resp, envelopeError("PartnerDebitRestrictionManagement", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
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
		return &resp, envelopeError("ResendOtp", resp.GetStatusCode(), resp.GetCode(), resp.Message, resp.Errors)
	}
	return &resp, nil
}

// LookupWalletByCustomerID looks up an existing Wema ALAT account by phone number to satisfy auth.WalletService.
func (a *Alat) LookupWalletByCustomerID(ctx context.Context, phoneOrID string) (*auth.WalletResponse, bool, error) {
	resp, err := a.GetPartnershipAccountDetails(ctx, phoneOrID)
	if err != nil {
		var alatErr *appErr.AlatError
		if errors.As(err, &alatErr) {
			log.Printf("[ALAT] level=INFO op=LookupWalletByCustomerID treating alat error as no account: %v", alatErr)
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

	// First check if account details already exist (e.g. a retried
	// registration job whose earlier attempt created the account). A
	// transient lookup error says nothing about whether the account exists,
	// so it must not lead on to creating a second one.
	existing, found, err := a.LookupWalletByCustomerID(ctx, walletInfo.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("alat account lookup failed: %w", err)
	}
	if found && existing != nil && existing.Wallet != nil && existing.Wallet.AccountNumber != "" {
		// Lift PND here too: an earlier attempt may have created the account
		// and then failed to lift it.
		if err := a.liftPnd(ctx, existing.Wallet.AccountNumber); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Trigger Step 1a account creation with BVN without OTP
	_, err = a.PostTier1BvnWithoutOtp(ctx, AlatTier1BvnWithoutOtpRequest{
		PhoneNumber:   walletInfo.PhoneNumber,
		Email:         walletInfo.Email,
		BVN:           walletInfo.BVN,
		CorrelationID: uuid.NewString(),
	})
	if err != nil {
		return nil, fmt.Errorf("alat wallet creation failed: %w", err)
	}

	// Poll for generated account
	const maxPollAttempts = 4
	pollInterval := 1000 * time.Millisecond
	for attempt := 1; attempt <= maxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		wallet, found, lookupErr := a.LookupWalletByCustomerID(ctx, walletInfo.PhoneNumber)
		if lookupErr == nil && found && wallet != nil && wallet.Wallet != nil && wallet.Wallet.AccountNumber != "" {
			// The account can't be debited until PND is lifted. On failure
			// the caller retries, finds the account via the existing-account
			// path above, and lifts PND again there.
			if err := a.liftPnd(ctx, wallet.Wallet.AccountNumber); err != nil {
				return nil, err
			}
			return wallet, nil
		}
	}

	log.Printf("[ALAT] level=WARN op=GenerateWallet account not visible after %d polls phone=%s - a retry should find it", maxPollAttempts, maskTail(walletInfo.PhoneNumber))
	return nil, errors.New("alat account generation pending; please check status shortly")
}

// liftPnd removes the post-no-debit restriction ALAT places on new accounts.
func (a *Alat) liftPnd(ctx context.Context, accountNumber string) error {
	if _, err := a.PartnerDebitRestrictionManagement(ctx, AlatPndRequest{
		PndType:       "LiftPnd",
		AccountNumber: accountNumber,
	}); err != nil {
		log.Printf("[ALAT] level=ERROR op=liftPnd account=%s: %v", accountNumber, err)
		return fmt.Errorf("alat lift pnd failed for account %s: %w", accountNumber, err)
	}
	return nil
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

	// Log the path only - the query string can carry the customer's phone.
	path := req.URL.Path
	started := time.Now()

	resp, err := a.Client.Do(req)
	if err != nil {
		log.Printf("[ALAT] level=ERROR http %s %s transport error after %s (ctx_err=%v): %v",
			method, path, time.Since(started).Round(time.Millisecond), ctx.Err(), err)
		return fmt.Errorf("alat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ALAT] level=ERROR http %s %s status=%d reading body failed: %v", method, path, resp.StatusCode, err)
		return fmt.Errorf("alat read response failed: %w", err)
	}
	elapsed := time.Since(started).Round(time.Millisecond)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ALAT] level=WARN http %s %s status=%d dur=%s request_id=%s body=%s",
			method, path, resp.StatusCode, elapsed, alatRequestID(resp.Header), truncateForLog(respBytes))
		var alatErr appErr.AlatError
		if json.Unmarshal(respBytes, &alatErr) == nil && (alatErr.Message != "" || alatErr.Code != "" || len(alatErr.Errors) > 0) {
			return &alatErr
		}
		return fmt.Errorf("alat request returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if target != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, target); err != nil {
			log.Printf("[ALAT] level=ERROR http %s %s status=%d dur=%s decode failed: %v body=%s",
				method, path, resp.StatusCode, elapsed, err, truncateForLog(respBytes))
			return fmt.Errorf("alat decode response failed: %w", err)
		}
	}

	return nil
}

// envelopeError logs and builds the error for an HTTP-2xx ALAT response whose
// envelope says status=false - the case doRequest can't see on its own.
func envelopeError(op, statusCode, code, message string, errs []string) *appErr.AlatError {
	log.Printf("[ALAT] level=WARN op=%s alat returned status=false: status_code=%s code=%s message=%q errors=%v",
		op, statusCode, code, message, errs)
	return &appErr.AlatError{StatusCode: statusCode, Code: code, Message: message, Errors: errs}
}

// alatRequestID picks out whichever correlation header the gateway sets, to
// quote to Wema when escalating a failed call.
func alatRequestID(h http.Header) string {
	for _, k := range []string{"X-Request-Id", "X-Correlation-Id", "Request-Id", "Apim-Request-Id"} {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return "-"
}

// truncateForLog caps response bodies in logs. Bodies are only logged on
// error/decode paths, which carry ALAT messages rather than account data.
func truncateForLog(b []byte) string {
	const max = 512
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}

func maskTail(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 4 {
		return v
	}
	return strings.Repeat("*", len(v)-4) + v[len(v)-4:]
}
