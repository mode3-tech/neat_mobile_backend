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
	"time"

)

type Optimus struct {
	BaseURL string
	ApiKey  string
	Client  *http.Client
}

func NewOptimus(baseURL string, apiKey string) *Optimus {
	return &Optimus{BaseURL: baseURL, ApiKey: apiKey, Client: &http.Client{Timeout: time.Second * 15}}
}

func (o *Optimus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	apiKey := strings.TrimSpace(o.ApiKey)
	baseURL := strings.TrimSpace(o.BaseURL)

	if apiKey == "" || baseURL == "" {
		log.Printf("Optimus is not configured\n")
		return nil, fmt.Errorf("Optimus is not configured\n")
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
		log.Printf("Failed to encode payload with json.Marshal: %s\n", err)
		return nil, fmt.Errorf("Failed to encode payload with json.Marshal: %s\n", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to create new request with context: %s\n", err)
		return nil, fmt.Errorf("Failed to create new request with context: %s\n", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
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

	// Optimus returns account data nested in data.data rather than top-level
	// customer/wallet objects. Map them into the shared shape so the rest of
	// the registration flow (normalizeWalletResponse, buildWalletRecordFromSnapshot)
	// can work without special-casing the provider.
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
		// Optimus has no separate wallet ID; use the customer ID so the
		// non-empty check in normalizeWalletResponse passes.
		WalletId: sub.CustomerID,
	}

	return &result, nil
}

func (o *Optimus) LookupWalletByCustomerID(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
	return nil, true, nil
}
