package tendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/providers/bvn"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	tendarBVNURL         = "https://api.tendar.co/onboarding/api/v1/kyc/nigeria/bvn/lookup"
	tendarMaxRetries     = 2
	tendarRetryBaseDelay = 200 * time.Millisecond
	tendarErrorBodyLimit = 2048
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
	if strings.TrimSpace(t.apiKey) == "" {
		log.Printf("tendar_bvn request skipped: Tendar API key is not configured")
		return nil, &appErr.TendarError{
			Status:  http.StatusInternalServerError,
			Code:    "CLIENT_ERROR",
			Message: "BVN validation service is not configured",
		}
	}

	body, err := json.Marshal(map[string]any{
		"bvn":      BVN,
		"send_otp": false,
	})
	if err != nil {
		log.Printf("tendar_bvn payload marshal failed: %v", err)
		return nil, &appErr.TendarError{Status: http.StatusInternalServerError, Code: "CLIENT_ERROR", Message: "BVN validation request could not be created"}
	}

	var lastErr error
	for attempt := 0; attempt <= tendarMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tendarBVNURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("tendar_bvn request creation failed: %v", err)
			return nil, &appErr.TendarError{Status: http.StatusInternalServerError, Code: "CLIENT_ERROR", Message: "BVN validation request could not be created"}
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(t.apiKey))

		start := time.Now()
		resp, err := t.httpClient.Do(req)
		duration := time.Since(start)
		if err != nil {
			lastErr = tendarRequestError(err)
			log.Printf("tendar_bvn request failed attempt=%d/%d duration=%s err=%v", attempt+1, tendarMaxRetries+1, duration, err)
			if attempt < tendarMaxRetries && shouldRetryRequestError(err) && waitForRetry(ctx, tendarRetryBaseDelay*time.Duration(attempt+1)) {
				continue
			}
			if ctx.Err() != nil {
				return nil, tendarRequestError(ctx.Err())
			}
			return nil, lastErr
		}

		log.Printf("tendar_bvn request completed attempt=%d/%d status=%d duration=%s", attempt+1, tendarMaxRetries+1, resp.StatusCode, duration)
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			providerErr := tendarHTTPError(resp, duration)
			resp.Body.Close()
			lastErr = providerErr
			if attempt < tendarMaxRetries && providerErr.Retryable && waitForRetry(ctx, tendarRetryBaseDelay*time.Duration(attempt+1)) {
				continue
			}
			if ctx.Err() != nil {
				return nil, tendarRequestError(ctx.Err())
			}
			return nil, providerErr
		}

		var result bvn.TendarBVNValidationSuccessResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			log.Printf("tendar_bvn response decode failed duration=%s err=%v", duration, decodeErr)
			return nil, &appErr.TendarError{Status: http.StatusBadGateway, Code: "INVALID_RESPONSE", Message: "BVN validation service returned an invalid response", Retryable: true}
		}
		return &result, nil
	}

	return nil, lastErr
}

func (t *Tendar) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvn.TendarBVNValidationSuccessResponse, error) {
	return t.validWithTendar(ctx, bvn)
}

func tendarHTTPError(resp *http.Response, duration time.Duration) *appErr.TendarError {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, tendarErrorBodyLimit))
	if readErr != nil {
		log.Printf("tendar_bvn error response read failed status=%d duration=%s err=%v", resp.StatusCode, duration, readErr)
	} else {
		log.Printf("tendar_bvn unexpected status=%d duration=%s response=%s", resp.StatusCode, duration, strings.TrimSpace(string(body)))
	}
	return &appErr.TendarError{
		Status:    resp.StatusCode,
		Code:      "HTTP_ERROR",
		Message:   "BVN validation service returned an unexpected error",
		Retryable: resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError,
	}
}

func tendarRequestError(err error) *appErr.TendarError {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &appErr.TendarError{Status: http.StatusRequestTimeout, Code: "TIMEOUT", Message: "BVN validation service timed out", Retryable: errors.Is(err, context.DeadlineExceeded)}
	}
	return &appErr.TendarError{Status: http.StatusBadGateway, Code: "NETWORK_ERROR", Message: "BVN validation service is unavailable", Retryable: true}
}

// shouldRetryRequestError reports whether a transport failure is worth another
// attempt. A cancelled context never is; a deadline or any net.Error (timeout,
// connection refused, DNS failure) may succeed on retry. Anything else — a
// malformed URL, a redirect policy rejection — will fail identically, so it is not
// retried.
func shouldRetryRequestError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
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
