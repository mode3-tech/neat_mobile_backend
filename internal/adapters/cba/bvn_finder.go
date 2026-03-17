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

type internalCustomerMatchResponse struct {
	Status  string                            `json:"status"`
	Message string                            `json:"message"`
	Data    loanproduct.CoreCustomerMatchData `json:"data"`
}

type internalErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (c *ProviderClient) MatchCustomerByBVN(ctx context.Context, bvn string) (*loanproduct.CoreCustomerMatchData, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("cba base url is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("cba internal key is not configured")
	}

	bvn = strings.TrimSpace(bvn)
	if bvn == "" {
		return nil, fmt.Errorf("bvn is required")
	}

	endpoint := c.baseURL + "/internal/customers/match-by-bvn?bvn=" + url.QueryEscape(bvn)

	fmt.Println(endpoint)

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
			return nil, fmt.Errorf("cba match customer by bvn failed with status %d: %s", resp.StatusCode, strings.TrimSpace(out.Message))
		}
		return nil, fmt.Errorf("cba match customer by bvn failed with status %d", resp.StatusCode)
	}

	var out internalCustomerMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out.Data, nil
}
