package cba

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"neat_mobile_app_backend/internal"
	"net/http"
	"net/url"
	"strings"
)

func (c *ProviderClient) UpdateCBACustomerBankInfo(ctx context.Context, coreCustomerID string, customerUpdate *internal.CustomerUpdateRequest) (*internal.CustomerUpdateResponse, error) {
	if strings.TrimRight(strings.TrimSpace(c.baseURL), "/") == "" {
		return nil, errors.New("cba base url is missing")
	}

	if strings.TrimSpace(c.baseURL) == "" {
		return nil, errors.New("cba api key is not configured")
	}

	if strings.TrimSpace(coreCustomerID) == "" {
		return nil, errors.New("core customer for cba customer is missing")
	}

	body, err := json.Marshal(customerUpdate)
	if err != nil {
		return nil, fmt.Errorf("error occured when marshalling payload for cba customer update: %s", err)
	}

	endpoint := c.baseURL + "/internal/customers/" + url.PathEscape(coreCustomerID) + "/wallet"
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error occured when creating new request for cba customer update: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorizatin", "bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cba customer update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return nil, fmt.Errorf("cba customer update failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("cba customer failed: %s", err)
	}

	var result internal.CustomerUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode cba customer update response: %w", err)
	}

	return &result, nil
}
