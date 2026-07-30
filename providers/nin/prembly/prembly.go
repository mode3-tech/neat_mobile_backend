package prembly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"net/http"
	"strings"
	"time"
)

const premblyErrorBodyLimit = 2048

type Prembly struct {
	apiKey     string
	httpClient *http.Client
}

// premblyErrorResponse supports documented and top-level Prembly errors.
type premblyErrorResponse struct {
	ResponseCode string `json:"response_code"`
	Message      string `json:"message"`
	Detail       string `json:"detail"`
	RequestID    string `json:"request_id"`
	Error        struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func NewPrembly(apiKey string) *Prembly {
	return &Prembly{apiKey: apiKey, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (p *Prembly) ValidateNIN(ctx context.Context, nin string) (*PremblyNINValidationSuccessResponse, error) {
	resp, duration, err := p.postJSON(ctx, "https://api.prembly.com/verification/vnin", "prembly_nin", map[string]string{
		"number_nin": nin,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, premblyHTTPError("prembly_nin", resp, duration)
	}

	var result PremblyNINValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("prembly_nin response decode failed duration=%s err=%v", duration, err)
		return nil, invalidPremblyResponseError()
	}
	return &result, nil
}

func (p *Prembly) ValidateNINWithFace(ctx context.Context, image, numberNin, dateOfBirth string) (*PremblyNINWithFaceValidationSuccessResponse, error) {
	resp, duration, err := p.postJSON(ctx, "https://api.prembly.com/verification/nin_w_face", "prembly_nin_with_face", map[string]string{
		"number_nin":    numberNin,
		"image":         image,
		"date_of_birth": dateOfBirth,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, premblyHTTPError("prembly_nin_with_face", resp, duration)
	}

	var result PremblyNINWithFaceValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("prembly_nin_with_face response decode failed duration=%s err=%v", duration, err)
		return nil, invalidPremblyResponseError()
	}
	return &result, nil
}

func (p *Prembly) postJSON(ctx context.Context, endpoint, operation string, payload map[string]string) (*http.Response, time.Duration, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		log.Printf("%s request skipped: Prembly API key is not configured", operation)
		return nil, 0, &appErr.PremblyError{
			Status:  http.StatusInternalServerError,
			Code:    "CLIENT_ERROR",
			Message: "NIN validation service is not configured",
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("%s payload marshal failed: %v", operation, err)
		return nil, 0, &appErr.PremblyError{
			Status:  http.StatusInternalServerError,
			Code:    "CLIENT_ERROR",
			Message: "NIN validation request could not be created",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("%s request creation failed: %v", operation, err)
		return nil, 0, &appErr.PremblyError{
			Status:  http.StatusInternalServerError,
			Code:    "CLIENT_ERROR",
			Message: "NIN validation request could not be created",
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(p.apiKey))

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Printf("%s request timed out duration=%s err=%v", operation, duration, err)
			return nil, duration, &appErr.PremblyError{
				Status:    http.StatusRequestTimeout,
				Code:      "TIMEOUT",
				Message:   "NIN validation service timed out",
				Retryable: true,
			}
		}
		log.Printf("%s request failed duration=%s err=%v", operation, duration, err)
		return nil, duration, &appErr.PremblyError{
			Status:    http.StatusBadGateway,
			Code:      "NETWORK_ERROR",
			Message:   "NIN validation service is unavailable",
			Retryable: true,
		}
	}
	return resp, duration, nil
}

func invalidPremblyResponseError() error {
	return &appErr.PremblyError{
		Status:    http.StatusBadGateway,
		Code:      "INVALID_RESPONSE",
		Message:   "NIN validation service returned an invalid response",
		Retryable: true,
	}
}

func premblyHTTPError(operation string, resp *http.Response, duration time.Duration) error {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, premblyErrorBodyLimit))
	if readErr != nil {
		log.Printf("%s error response read failed status=%d duration=%s err=%v", operation, resp.StatusCode, duration, readErr)
	} else {
		log.Printf("%s unexpected status=%d duration=%s response=%s", operation, resp.StatusCode, duration, strings.TrimSpace(string(body)))
	}

	providerErr := &appErr.PremblyError{
		Status:    resp.StatusCode,
		Code:      "HTTP_ERROR",
		Message:   "NIN validation service returned an unexpected error",
		Retryable: resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError,
	}
	var response premblyErrorResponse
	if len(body) > 0 && json.Unmarshal(body, &response) == nil {
		providerErr.RequestID = response.RequestID
		if response.Error.Code != "" {
			providerErr.Code = response.Error.Code
		} else if response.ResponseCode != "" {
			providerErr.Code = response.ResponseCode
		}
		if response.Error.Message != "" {
			providerErr.Message = response.Error.Message
		} else if response.Message != "" {
			providerErr.Message = response.Message
		} else if response.Detail != "" {
			providerErr.Message = response.Detail
		}
	}
	return providerErr
}
