package cba

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neat_mobile_app_backend/modules/loanproduct"
	"net/http"
	"strings"
)

type manualRepaymentResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (c *ProviderClient) MakeManualRepayment(ctx context.Context, repaymentReq loanproduct.RepaymentRequest) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("cba base url is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return fmt.Errorf("cba internal key is not configured")
	}

	amount := repaymentReq.Amount
	if amount < 0 {
		return fmt.Errorf("amount too low for repayment")
	}
	amount = amount * 100

	repaymentID := repaymentReq.RepaymentID
	if strings.TrimSpace(repaymentID) == "" {
		return fmt.Errorf("invalid repayment id")
	}

	payload := loanproduct.RepaymentRequest{
		Amount:      amount,
		RepaymentID: repaymentID,
	}

	endpoint := strings.TrimSpace(c.baseURL) + "/internal/repayments/pay/"

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-API-Key", strings.TrimSpace(c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("manual repayment request failed: %w", err)
	}
	defer resp.Body.Close()

	var result manualRepaymentResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode repayment response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(result.Message)
		if msg == "" {
			msg = "repayment failed"
		}
		log.Println(err)

		return fmt.Errorf("%s", msg)
	}

	return nil
}
