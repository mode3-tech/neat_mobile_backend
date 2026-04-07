package providus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/wallet"
	"net/http"
	"strings"
	"time"
)

var phoneAreaCodes = []string{"0701", "0703", "0706", "0801", "0802", "0803", "0805", "0806", "0807", "0810", "0811", "0812", "0813", "0814", "0815", "0816", "0817", "0818", "0819", "0901", "0902", "0903", "0904", "0905", "0906", "0907", "0908", "0909"}

func randomDigits(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('0' + rand.Intn(10))
	}
	return string(b)
}

func randomBVN() string {
	return string(byte('1'+rand.Intn(9))) + randomDigits(10)
}

func randomPhone() string {
	return phoneAreaCodes[rand.Intn(len(phoneAreaCodes))] + randomDigits(7)
}

func randomName(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	b[0] = letters[rand.Intn(len(letters))] - 32 // uppercase first letter
	for i := 1; i < length; i++ {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type Providus struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

func NewProvidus(apiKey, baseURL string) *Providus {
	return &Providus{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func extractErrorMessage(body []byte) string {
	var result struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Message != "" {
		return result.Message
	}
	return strings.TrimSpace(string(body))
}

func (p *Providus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/wallet"

	firstName := randomName(5 + rand.Intn(4))
	lastName := randomName(5 + rand.Intn(4))
	walletInfo.BVN = randomBVN()
	walletInfo.PhoneNumber = randomPhone()
	walletInfo.FirstName = firstName
	walletInfo.LastName = lastName
	walletInfo.Metadata = map[string]interface{}{"customer_id": "user124"}
	walletInfo.Address = "123 Main St, Ibadan"
	walletInfo.Email = strings.ToLower(firstName+"."+lastName) + "@example.com"
	walletInfo.DateOfBirth = "1991-01-01"

	body, err := json.Marshal(walletInfo)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("providus wallet request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("providus wallet generation failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("providus wallet generation failed: %s", extractErrorMessage(respBody))
	}

	var result auth.WalletResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus wallet generation response: %w", err)
	}

	return &result, nil
}

func (p *Providus) FetchBanks(ctx context.Context) ([]wallet.Bank, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/transfer/banks"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("providus banks request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("providus banks fetch failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("providus banks fetch failed: %s", extractErrorMessage(respBody))
	}

	var result wallet.BankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus banks response: %w", err)
	}

	return result.Banks, nil
}

func (p *Providus) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*wallet.BankDetails, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/transfer/account/details?sortCode=" + bankCode + "&accountNumber=" + accountNumber

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("providus bank details request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("providus bank details fetch failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("providus bank details fetch failed: %s", extractErrorMessage(respBody))
	}
	defer resp.Body.Close()

	var result wallet.BankDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus bank details response: %w", err)
	}

	return &result.Account, nil
}

func (p *Providus) InitiateTransfer(ctx context.Context, providusCustomerID string, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/transfer/bank"

	payload := map[string]any{
		"amount":        transferInfo.Amount,
		"sortCode":      transferInfo.SortCode,
		"narration":     transferInfo.Narration,
		"accountNumber": transferInfo.AccountNumber,
		"customerId":    providusCustomerID,
		"accountName":   transferInfo.AccountName,
		"metadata":      transferInfo.Metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("providus transfer request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("providus transfer failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("providus transfer failed: %s", extractErrorMessage(respBody))
	}

	var result wallet.TransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus transfer response: %w", err)
	}

	return &result, nil
}
