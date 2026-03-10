package tendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/providers/bvn"
	"net"
	"net/http"
	"time"
)

type Tendar struct {
	apiKey     string
	httpClient *http.Client
}

func NewTendar(apiKey string) *Tendar {
	return &Tendar{apiKey: apiKey, httpClient: &http.Client{
		Timeout: 10 * time.Second,
	}}
}

func (t *Tendar) validWithTendar(ctx context.Context, BVN string) (*bvn.TendarBVNValidationSuccessResponse, error) {
	const maxRetries = 2
	const retryBaseDelay = 200 * time.Millisecond
	url := "https://api.tendar.co/onboarding/api/v1/kyc/nigeria/bvn/lookup"

	payload := map[string]interface{}{
		"bvn":      BVN,
		"send_otp": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
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
			log.Printf("tendar_bvn request failed attempt=%d/%d duration=%s err=%v", attempt+1, maxRetries+1, duration, err)
			if attempt < maxRetries && shouldRetryRequestError(err) {
				if !waitForRetry(ctx, retryBaseDelay*time.Duration(attempt+1)) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, err
		}

		log.Printf("tendar_bvn request completed attempt=%d/%d status=%d duration=%s", attempt+1, maxRetries+1, resp.StatusCode, duration)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			log.Printf("tendar_bvn non-2xx status=%d duration=%s attempt=%d/%d", resp.StatusCode, duration, attempt+1, maxRetries+1)

			if attempt < maxRetries && shouldRetryStatusCode(resp.StatusCode) {
				if !waitForRetry(ctx, retryBaseDelay*time.Duration(attempt+1)) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("tendar bvn validation failed with status %d", resp.StatusCode)
		}

		var result bvn.TendarBVNValidationSuccessResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			log.Printf("tendar_bvn response decode failed duration=%s err=%v", duration, err)
			return nil, err
		}
		resp.Body.Close()
		return &result, nil
	}

	return nil, fmt.Errorf("tendar bvn validation failed after retries")
}

func (t *Tendar) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvn.TendarBVNValidationSuccessResponse, error) {
	return t.validWithTendar(ctx, bvn)
}

func shouldRetryStatusCode(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func shouldRetryRequestError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return true
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
