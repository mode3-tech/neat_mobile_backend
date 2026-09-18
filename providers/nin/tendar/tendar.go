package tendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	nin "neat_mobile_app_backend/providers/nin"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	tendarNINURL         = "https://api.tendar.co/onboarding/api/v1/kyc/nigeria/nin/lookup"
	tendarMaxRetries     = 2
	tendarRetryBaseDelay = 200 * time.Millisecond
	tendarErrorBodyLimit = 2048
)

type Tendar struct {
	apiKey     string
	httpClient *http.Client
}

type lookupResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    struct {
		Details struct {
			FirstName   string `json:"first_name"`
			LastName    string `json:"last_name"`
			MiddleName  string `json:"middle_name"`
			DateOfBirth string `json:"date_of_birth"`
			PhoneNumber string `json:"phone_number"`
			Email       string `json:"email"`
		} `json:"details"`
	} `json:"data"`
}

func NewTendar(apiKey string) *Tendar {
	return &Tendar{apiKey: apiKey, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (t *Tendar) ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		log.Printf("tendar_nin request skipped: Tendar API key is not configured")
		return nil, &appErr.TendarError{
			Status:  http.StatusInternalServerError,
			Code:    "CLIENT_ERROR",
			Message: "NIN validation service is not configured",
		}
	}

	body, err := json.Marshal(map[string]any{"nin": number, "send_otp": false})
	if err != nil {
		log.Printf("tendar_nin payload marshal failed: %v", err)
		return nil, &appErr.TendarError{Status: http.StatusInternalServerError, Code: "CLIENT_ERROR", Message: "NIN validation request could not be created"}
	}

	var lastErr error
	for attempt := 0; attempt <= tendarMaxRetries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, tendarNINURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("tendar_nin request creation failed: %v", err)
			return nil, &appErr.TendarError{Status: http.StatusInternalServerError, Code: "CLIENT_ERROR", Message: "NIN validation request could not be created"}
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(t.apiKey))

		start := time.Now()
		response, err := t.httpClient.Do(request)
		duration := time.Since(start)
		if err != nil {
			lastErr = tendarRequestError(err)
			log.Printf("tendar_nin request failed attempt=%d/%d duration=%s err=%v", attempt+1, tendarMaxRetries+1, duration, err)
			if attempt < tendarMaxRetries && shouldRetryRequestError(err) && waitForRetry(ctx, tendarRetryBaseDelay*time.Duration(attempt+1)) {
				continue
			}
			if ctx.Err() != nil {
				return nil, tendarRequestError(ctx.Err())
			}
			return nil, lastErr
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			providerErr := tendarHTTPError(response, duration)
			response.Body.Close()
			lastErr = providerErr
			if attempt < tendarMaxRetries && providerErr.Retryable && waitForRetry(ctx, tendarRetryBaseDelay*time.Duration(attempt+1)) {
				continue
			}
			if ctx.Err() != nil {
				return nil, tendarRequestError(ctx.Err())
			}
			return nil, providerErr
		}

		var result lookupResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if decodeErr != nil {
			log.Printf("tendar_nin response decode failed duration=%s err=%v", duration, decodeErr)
			return nil, &appErr.TendarError{Status: http.StatusBadGateway, Code: "INVALID_RESPONSE", Message: "NIN validation service returned an invalid response", Retryable: true}
		}
		if result.Error {
			message := strings.TrimSpace(result.Message)
			if message == "" {
				message = "NIN validation service rejected the request"
			}
			return nil, &appErr.TendarError{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: message}
		}

		return &nin.ValidationResponse{Data: nin.ValidationData{
			FirstName:   result.Data.Details.FirstName,
			MiddleName:  result.Data.Details.MiddleName,
			Surname:     result.Data.Details.LastName,
			BirthDate:   result.Data.Details.DateOfBirth,
			TelephoneNo: result.Data.Details.PhoneNumber,
			Email:       result.Data.Details.Email,
		}}, nil
	}

	return nil, lastErr
}

// func (t *Tendar) ValidateNINWithFace(ctx context.Context, )

func tendarHTTPError(response *http.Response, duration time.Duration) *appErr.TendarError {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, tendarErrorBodyLimit))
	if readErr != nil {
		log.Printf("tendar_nin error response read failed status=%d duration=%s err=%v", response.StatusCode, duration, readErr)
	} else {
		log.Printf("tendar_nin unexpected status=%d duration=%s response=%s", response.StatusCode, duration, strings.TrimSpace(string(body)))
	}
	return &appErr.TendarError{
		Status:    response.StatusCode,
		Code:      "HTTP_ERROR",
		Message:   "NIN validation service returned an unexpected error",
		Retryable: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError,
	}
}

func tendarRequestError(err error) *appErr.TendarError {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &appErr.TendarError{Status: http.StatusRequestTimeout, Code: "TIMEOUT", Message: "NIN validation service timed out", Retryable: errors.Is(err, context.DeadlineExceeded)}
	}
	return &appErr.TendarError{Status: http.StatusBadGateway, Code: "NETWORK_ERROR", Message: "NIN validation service is unavailable", Retryable: true}
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
