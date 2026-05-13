package baas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neat_mobile_app_backend/internal/modules/auth"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Optimus struct {
	BaseURL      string
	Username     string
	Password     string
	Client       *http.Client
	mu           sync.Mutex
	cachedToken  string
	tokenExpiry  time.Time
}

func NewOptimus(baseURL, username, password string) *Optimus {
	return &Optimus{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   &http.Client{Timeout: time.Second * 15},
	}
}

// getToken returns a valid access token, generating a new one if the cached
// token is absent or within 60 seconds of expiry.
func (o *Optimus) getToken(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cachedToken != "" && time.Now().Add(60*time.Second).Before(o.tokenExpiry) {
		return o.cachedToken, nil
	}

	url := strings.TrimSpace(o.BaseURL) + "/tokens/generate"
	body, err := json.Marshal(optimusTokenRequest{
		Username: strings.TrimSpace(o.Username),
		Password: strings.TrimSpace(o.Password),
	})
	if err != nil {
		return "", fmt.Errorf("optimus: marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("optimus: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: token generation request failed: %v", err)
		return "", fmt.Errorf("optimus: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("optimus: token generation failed status=%d body=%s", resp.StatusCode, respBody)
		return "", fmt.Errorf("optimus: token generation failed with status %d", resp.StatusCode)
	}

	var result optimusTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("optimus: decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("optimus: token response contained empty accessToken")
	}

	o.cachedToken = result.AccessToken
	// Tokens appear to live ~30 minutes; treat them as valid for 25 minutes so
	// we refresh before actual expiry without needing to parse the JWT.
	o.tokenExpiry = time.Now().Add(25 * time.Minute)
	log.Printf("optimus: access token refreshed, valid until %s", o.tokenExpiry.Format(time.RFC3339))

	return o.cachedToken, nil
}

func (o *Optimus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	baseURL := strings.TrimSpace(o.BaseURL)
	if baseURL == "" || strings.TrimSpace(o.Username) == "" {
		log.Printf("Optimus is not configured")
		return nil, fmt.Errorf("Optimus is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("Optimus: failed to obtain access token: %v", err)
		return nil, err
	}

	url := baseURL + "/Customer/create-by-bvn"
	payload := OptimusPayload{
		RequestId:         walletInfo.RequestID,
		Email:             walletInfo.Email,
		Gender:            walletInfo.Gender,
		MaritalStatus:     walletInfo.MaritalStatus,
		MothersMaidenName: walletInfo.MothersMaidenName,
		Address:           walletInfo.Address,
		HouseNo:           walletInfo.HouseNo,
		ProductId:         walletInfo.ProductId,
		PhoneNumber:       walletInfo.PhoneNumber,
		BVN:               walletInfo.BVN,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to encode payload with json.Marshal: %s", err)
		return nil, fmt.Errorf("Failed to encode payload with json.Marshal: %s", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to create new request with context: %s", err)
		return nil, fmt.Errorf("Failed to create new request with context: %s", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Accept", "text/plain")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("Optimus wallet generation request failed: %v", err)
		return nil, fmt.Errorf("Optimus wallet request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			log.Printf("Optimus wallet generation failed with status: %d", resp.StatusCode)
			return nil, fmt.Errorf("Optimus wallet generation failed with status: %d", resp.StatusCode)
		}
		log.Printf("Optimus wallet generation failed: %s", extractErrorMessage(respBody))
		return nil, fmt.Errorf("Optimus wallet generation failed: %s", extractErrorMessage(respBody))
	}

	var result auth.WalletResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode Optimus wallet generation response: %v", err)
		return nil, fmt.Errorf("Failed to decode Optimus wallet generation response: %w", err)
	}

	sub := result.Data.Data
	result.Customer = &auth.WalletCustomer{
		ID:       sub.CustomerID,
		Currency: "NGN",
	}
	result.Wallet = &auth.WalletInfo{
		AccountNumber: sub.NUBAN,
		AccountName:   sub.AccountName,
		BankCode:      "000036",
		BankName:      "OPTIMUS BANK",
		WalletId:      sub.CustomerID,
	}

	return &result, nil
}

func (o *Optimus) LookupWalletByCustomerID(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
	return nil, true, nil
}
