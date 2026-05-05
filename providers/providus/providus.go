package providus

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/wallet"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

func BuildWalletPayloadSeed(secretKey string, walletInfo *auth.WalletPayload) (string, error) {
	if walletInfo == nil {
		return "", errors.New("wallet payload is required")
	}
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return "", nil
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(strings.TrimSpace(walletInfo.BVN)))
	h.Write([]byte("|"))
	h.Write([]byte(strings.ToLower(strings.TrimSpace(walletInfo.Email))))
	h.Write([]byte("|"))
	h.Write([]byte(strings.TrimSpace(walletInfo.PhoneNumber)))
	h.Write([]byte("|"))
	h.Write([]byte(strings.TrimSpace(walletInfo.DateOfBirth)))

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:10], nil
}

func SeedWalletPayload(walletInfo *auth.WalletPayload, secretKey string, usePrefix bool) (*auth.WalletPayload, error) {
	if walletInfo == nil {
		return nil, errors.New("wallet payload is required")
	}

	seed, err := BuildWalletPayloadSeed(secretKey, walletInfo)
	if err != nil {
		return nil, err
	}

	copyPayload := *walletInfo
	copyPayload.Metadata = make(map[string]interface{})
	for k, v := range walletInfo.Metadata {
		copyPayload.Metadata[k] = v
	}

	if seed == "" {
		return &copyPayload, nil
	}

	copyPayload.FirstName = decorateStringWithSeed(walletInfo.FirstName, seed, usePrefix)
	copyPayload.LastName = decorateStringWithSeed(walletInfo.LastName, seed, usePrefix)
	copyPayload.PhoneNumber = decoratePhoneNumberWithSeed(walletInfo.PhoneNumber, seed, usePrefix)
	copyPayload.BVN = decorateBVNWithSeed(seed)
	copyPayload.Email = decorateEmailWithSeed(walletInfo.Email, seed, usePrefix)
	copyPayload.Address = decorateStringWithSeed(walletInfo.Address, seed, usePrefix)
	if copyPayload.Metadata == nil {
		copyPayload.Metadata = map[string]interface{}{}
	}
	copyPayload.Metadata["wallet_generation_seed"] = seed

	return &copyPayload, nil
}

func decorateBVNWithSeed(seed string) string {
	return normalizeSeedDigits(seed, 11)
}

func decoratePhoneNumberWithSeed(phone, seed string, usePrefix bool) string {
	if seed == "" {
		return normalizePhoneTo234(phone)
	}

	seedDigits := normalizeSeedDigits(seed, 10)
	return "234" + seedDigits
}

func normalizePhoneTo234(phone string) string {
	digits := normalizeDigits(phone)
	if strings.HasPrefix(digits, "234") {
		return digits
	}
	if len(digits) > 10 {
		digits = digits[len(digits)-10:]
	}
	return "234" + digits
}

func normalizeDigits(value string) string {
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

func truncatePhoneNumber(phone string, maxLen int, usePrefix bool) string {
	if len(phone) <= maxLen {
		return phone
	}
	if usePrefix {
		return phone[:maxLen]
	}
	return phone[len(phone)-maxLen:]
}

func normalizeSeedDigits(seed string, length int) string {
	var digits strings.Builder
	for _, ch := range seed {
		if digits.Len() >= length {
			break
		}
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
			continue
		}
		if ch >= 'a' && ch <= 'f' {
			digits.WriteRune('0' + ((ch - 'a') % 10))
			continue
		}
		if ch >= 'A' && ch <= 'F' {
			digits.WriteRune('0' + ((ch - 'A') % 10))
			continue
		}
	}
	for digits.Len() < length {
		digits.WriteByte('0')
	}
	return digits.String()[:length]
}

func decorateEmailWithSeed(email, seed string, usePrefix bool) string {
	email = strings.TrimSpace(email)
	if email == "" || seed == "" {
		return email
	}

	parts := strings.SplitN(email, "@", 2)
	local := parts[0]
	domain := ""
	if len(parts) == 2 {
		domain = parts[1]
	}

	if usePrefix {
		local = seed + "." + local
	} else {
		local = local + "+" + seed
	}

	if domain == "" {
		return local
	}
	return local + "@" + domain
}

func decorateStringWithSeed(value, seed string, usePrefix bool) string {
	value = strings.TrimSpace(value)
	if value == "" || seed == "" {
		return value
	}

	if usePrefix {
		return seed + "-" + value
	}
	return value + "-" + seed
}

func (p *Providus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/wallet"

	body, err := json.Marshal(walletInfo)
	log.Printf("Providus wallet generation request payload: %s", string(body))
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
		log.Printf("providus wallet generation request failed: %v", err)
		return nil, fmt.Errorf("providus wallet request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			log.Printf("providus wallet generation failed with status: %d", resp.StatusCode)
			return nil, fmt.Errorf("providus wallet generation failed with status: %d", resp.StatusCode)
		}
		log.Printf("providus wallet generation failed: %s", extractErrorMessage(respBody))
		return nil, fmt.Errorf("providus wallet generation failed: %s", extractErrorMessage(respBody))
	}

	var result auth.WalletResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {

		return nil, fmt.Errorf("failed to decode providus wallet generation response: %w", err)
	}

	return &result, nil
}

func (p *Providus) LookupWalletByCustomerID(ctx context.Context, walletCustomerID string) (*auth.WalletResponse, bool, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		log.Print("providus service not configured")
		return nil, false, errors.New("providus service not configured")
	}

	requestedCustomerID := strings.TrimSpace(walletCustomerID)
	if requestedCustomerID == "" {
		log.Print("providus customer id is required")
		return nil, false, errors.New("providus customer id is required")
	}

	endpoint := p.BaseURL + "/wallet/customer?customerId=" + url.QueryEscape(requestedCustomerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		log.Printf("providus wallet lookup request failed: %v", err)
		return nil, false, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		log.Printf("providus wallet lookup request failed: %v", err)
		return nil, false, fmt.Errorf("providus wallet lookup request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			log.Printf("providus wallet lookup failed with status: %d", resp.StatusCode)
			return nil, false, fmt.Errorf("providus wallet lookup failed with status: %d", resp.StatusCode)
		}
		log.Printf("providus wallet lookup failed: %s", extractErrorMessage(respBody))
		return nil, false, fmt.Errorf("providus wallet lookup failed: %s", extractErrorMessage(respBody))
	}

	var result struct {
		Status *bool `json:"status"`
		Wallet struct {
			ID                    string `json:"id"`
			Type                  string `json:"type"`
			Tier                  string `json:"tier"`
			Status                string `json:"status"`
			Email                 string `json:"email"`
			CustomerID            string `json:"customerId"`
			LastName              string `json:"lastName"`
			FirstName             string `json:"firstName"`
			BankName              string `json:"bankName"`
			BankCode              string `json:"bankCode"`
			CreatedAt             string `json:"createdAt"`
			UpdatedAt             string `json:"updatedAt"`
			AccountName           string `json:"accountName"`
			PhoneNumber           string `json:"phoneNumber"`
			AccountNumber         string `json:"accountNumber"`
			BookedBalance         int64  `json:"bookedBalance"`
			AvailableBalance      int64  `json:"availableBalance"`
			AccountReference      string `json:"accountReference"`
			DailyTransactionLimit int64  `json:"dailyTransactionLimit"`
		} `json:"wallet"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("failed to decode providus wallet lookup response: %v", err)
		return nil, false, fmt.Errorf("failed to decode providus wallet lookup response: %w", err)
	}

	resolvedCustomerID := strings.TrimSpace(result.Wallet.CustomerID)
	if resolvedCustomerID == "" {
		resolvedCustomerID = requestedCustomerID
	}

	mapped := &auth.WalletResponse{
		Status: result.Status,
		Customer: &auth.WalletCustomer{
			ID:          resolvedCustomerID,
			Metadata:    map[string]any{"customer_id": resolvedCustomerID},
			PhoneNumber: strings.TrimSpace(result.Wallet.PhoneNumber),
			LastName:    strings.TrimSpace(result.Wallet.LastName),
			FirstName:   strings.TrimSpace(result.Wallet.FirstName),
			Email:       strings.TrimSpace(result.Wallet.Email),
			Tier:        strings.TrimSpace(result.Wallet.Tier),
			UpdatedAt:   strings.TrimSpace(result.Wallet.UpdatedAt),
			CreatedAt:   strings.TrimSpace(result.Wallet.CreatedAt),
		},
		Wallet: &auth.WalletInfo{
			ID:               strings.TrimSpace(result.Wallet.ID),
			Email:            strings.TrimSpace(result.Wallet.Email),
			BankName:         strings.TrimSpace(result.Wallet.BankName),
			BankCode:         strings.TrimSpace(result.Wallet.BankCode),
			AccountName:      strings.TrimSpace(result.Wallet.AccountName),
			AccountNumber:    strings.TrimSpace(result.Wallet.AccountNumber),
			AccountReference: strings.TrimSpace(result.Wallet.AccountReference),
			UpdatedAt:        strings.TrimSpace(result.Wallet.UpdatedAt),
			CreatedAt:        strings.TrimSpace(result.Wallet.CreatedAt),
			BookedBalance:    result.Wallet.BookedBalance,
			AvailableBalance: result.Wallet.AvailableBalance,
			Status:           strings.TrimSpace(result.Wallet.Status),
			WalletType:       strings.TrimSpace(result.Wallet.Type),
			WalletId:         strings.TrimSpace(result.Wallet.ID),
		},
	}

	return mapped, true, nil
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
		log.Printf("providus banks request failed: %v", err)
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
		"amount":        float64(transferInfo.Amount) / 100,
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

func (p *Providus) InitiateBulkTransfer(ctx context.Context, req []wallet.BulkTransferRecipientInfo) (*wallet.ProvidusBatchTransferResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/transfer/bank/batch"

	type transferItem struct {
		Amount        float64        `json:"amount"`
		SortCode      string         `json:"sortCode"`
		Narration     *string        `json:"narration,omitempty"`
		AccountNumber string         `json:"accountNumber"`
		AccountName   *string        `json:"accountName,omitempty"`
		Metadata      map[string]any `json:"metadata,omitempty"`
	}

	payload := make([]transferItem, 0, len(req))
	for _, trfReq := range req {
		payload = append(payload, transferItem{
			Amount:        float64(trfReq.Amount) / 100,
			SortCode:      trfReq.SortCode,
			Narration:     trfReq.Narration,
			AccountNumber: trfReq.AccountNumber,
			AccountName:   trfReq.AccountName,
			Metadata:      trfReq.Metadata,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("providus bulk transfer request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("providus bulk transfer failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("providus bulk transfer failed: %s", extractErrorMessage(respBody))
	}

	var result wallet.ProvidusBatchTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus bulk transfer response: %w", err)
	}

	return &result, nil
}
