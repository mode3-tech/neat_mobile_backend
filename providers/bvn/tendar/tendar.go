package tendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neat_mobile_app_backend/providers/bvn"
	"net/http"
	"time"
)

type Tendar struct {
	apiKey     string
	httpClient *http.Client
}

func NewTendar(apiKey string) *Tendar {
	return &Tendar{apiKey: apiKey, httpClient: &http.Client{
		Timeout: time.Second * 30,
	}}
}

func (t *Tendar) validWithTendar(ctx context.Context, BVN string) (*bvn.TendarBVNValidationSuccessResponse, error) {

	url := "https://api.tendar.co/onboarding/api/v1/kyc/nigeria/bvn/lookup"

	payload := map[string]interface{}{
		"bvn":      BVN,
		"send_otp": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	start := time.Now()
	resp, err := t.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		log.Printf("tendar_bvn request failed duration=%s err=%v", duration, err)
		return nil, err
	}

	log.Printf("tendar_bvn request completed status=%d duration=%s", resp.StatusCode, duration)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("tendar_bvn non-2xx status=%d duration=%s", resp.StatusCode, duration)
		return nil, fmt.Errorf("tendar bvn validation failed with status %d", resp.StatusCode)
	}

	var result bvn.TendarBVNValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (t *Tendar) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvn.TendarBVNValidationSuccessResponse, error) {
	return t.validWithTendar(ctx, bvn)
}
