package cba

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"neat_mobile_app_backend/modules/loanproduct"
)

type internalCustomerLoansResponse struct {
	Status  string                             `json:"status"`
	Message string                             `json:"message"`
	Data    []loanproduct.CoreCustomerLoanItem `json:"data"`
}

type internalLoanDetailResponse struct {
	Status  string                     `json:"status"`
	Message string                     `json:"message"`
	Data    loanproduct.CoreLoanDetail `json:"data"`
}

func (c *ProviderClient) GetCustomerLoans(ctx context.Context, customerID string) ([]loanproduct.CoreCustomerLoanItem, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("cba base url is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("cba internal key is not configured")
	}

	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("customer id is required")
	}

	endpoint := c.baseURL + "/internal/customers/" + url.PathEscape(customerID) + "/loans"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var out internalErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && strings.TrimSpace(out.Message) != "" {
			return nil, fmt.Errorf("cba get customer loans failed with status %d: %s", resp.StatusCode, strings.TrimSpace(out.Message))
		}
		return nil, fmt.Errorf("cba get customer loans failed with status %d", resp.StatusCode)
	}

	var out internalCustomerLoansResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out.Data, nil
}

func (c *ProviderClient) GetLoanDetail(ctx context.Context, loanID string) (*loanproduct.CoreLoanDetail, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("cba base url is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("cba internal key is not configured")
	}

	loanID = strings.TrimSpace(loanID)
	if loanID == "" {
		return nil, fmt.Errorf("loan id is required")
	}

	endpoint := c.baseURL + "/internal/loans/" + url.PathEscape(loanID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var out internalErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && strings.TrimSpace(out.Message) != "" {
			return nil, fmt.Errorf("cba get loan detail failed with status %d: %s", resp.StatusCode, strings.TrimSpace(out.Message))
		}
		return nil, fmt.Errorf("cba get loan detail failed with status %d", resp.StatusCode)
	}

	var out internalLoanDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out.Data, nil
}
