package providus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"neat_mobile_app_backend/modules/auth"
	"neat_mobile_app_backend/modules/wallet"
	"net/http"
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

func (p *Providus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/wallet"

	walletInfo.BVN = "01234567891"

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
		return nil, fmt.Errorf("providus wallet generation failed with status: %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
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
		return nil, fmt.Errorf("providus banks fetch failed with status: %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
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
		return nil, fmt.Errorf("providus bank details fetch failed with status: %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	defer resp.Body.Close()

	var result wallet.BankDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus bank details response: %w", err)
	}

	return &result.Account, nil
}

func (p *Providus) InitiateTransfer(ctx context.Context, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	if strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return nil, errors.New("providus service not configured")
	}

	url := p.BaseURL + "/transfer/bank"

	body, err := json.Marshal(transferInfo)
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
		return nil, fmt.Errorf("providus transfer failed with status: %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result wallet.TransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode providus transfer response: %w", err)
	}

	return &result, nil
}
