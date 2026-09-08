package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
)

type SMSLive struct {
	APIKey   string
	BaseURL  string
	SenderID string
	Client   *http.Client
}

func NewSMSLive(apiKey, baseURL, senderID string) *SMSLive {
	return &SMSLive{
		APIKey:   apiKey,
		BaseURL:  baseURL,
		SenderID: senderID,
		Client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SMSLive) Send(ctx context.Context, destination, message string) error {
	if s.BaseURL == "" || s.APIKey == "" {
		log.Printf("smslive: baseURL or APIKey is not set")
		return appErr.SMSLiveErrorResponse{Code: int64(http.StatusInternalServerError), Message: "Service is not configured"}
	}

	url := s.BaseURL + "/sms"

	payload := map[string]any{
		"senderID":     strings.TrimSpace(s.SenderID),
		"messageText":  strings.TrimSpace(message),
		"mobileNumber": strings.TrimSpace(destination),
		"route":        "transactional",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("smslive: failed to marshal payload: %v", err)
		return appErr.SMSLiveErrorResponse{Code: int64(http.StatusInternalServerError), Message: "failed to marshal payload"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("smslive: failed to create request: %v", err)
		return appErr.SMSLiveErrorResponse{Code: int64(http.StatusInternalServerError), Message: "failed to create request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.Client.Do(req)
	if err != nil {
		log.Printf("smslive: failed to send request: %v", err)
		return appErr.SMSLiveErrorResponse{Code: int64(http.StatusInternalServerError), Message: "failed to send request"}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		bodyStr := strings.TrimSpace(string(respBody))

		msg := bodyStr
		var smsliveErrResponse smsLiveErrorResponse
		if json.Unmarshal([]byte(bodyStr), &smsliveErrResponse) == nil {
			msg = smsliveErrResponse.Message
		}

		log.Printf("smslive: request failed with status code %d", resp.StatusCode)
		return appErr.SMSLiveErrorResponse{Code: int64(resp.StatusCode), Message: msg}
	}

	return nil
}
