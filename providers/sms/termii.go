package termii

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

type SMS struct {
	apiKey     string
	senderID   string
	httpClient *http.Client
}

func NewSMSService(apiKey, senderID string) *SMS {
	return &SMS{apiKey: apiKey, senderID: senderID, httpClient: &http.Client{
		Timeout: 10 * time.Second,
	}}
}

func (s *SMS) Send(ctx context.Context, destination, message string) error {
	if strings.TrimSpace(s.apiKey) == "" || strings.TrimSpace(s.senderID) == "" {
		log.Println("sms service not configured")
		return &appErr.TermiiError{Code: 500, Message: "sms service not configured"}
	}

	url := "https://v3.api.termii.com/api/sms/send"

	payload := map[string]string{
		"api_key": strings.TrimSpace(s.apiKey),
		"from":    s.senderID,
		"to":      destination,
		"sms":     message,
		"type":    "plain",
		"channel": "generic",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("failed to marshal SMS payload:", err)
		return &appErr.TermiiError{Code: 500, Message: "failed to marshal SMS payload"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Println("failed to create SMS request:", err)
		return &appErr.TermiiError{Code: 500, Message: "failed to create SMS request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Println("SMS provider request timed out")
			return &appErr.TermiiError{Code: 408, Message: "SMS provider request timed out"}
		}
		log.Println("SMS provider request failed:", err)
		return &appErr.TermiiError{Code: 500, Message: "SMS provider request failed"}
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		bodyStr := strings.TrimSpace(string(respBody))

		var termiiErrResp TermiiErrorResponse
		code := resp.StatusCode
		msg := bodyStr
		if json.Unmarshal([]byte(bodyStr), &termiiErrResp) == nil {
			code = termiiErrResp.Code
			msg = termiiErrResp.Message
		}
		return &appErr.TermiiError{
			Code:    code,
			Message: msg,
			Status:  termiiErrResp.Status,
			Link:    termiiErrResp.Link,
		}
	}

	return nil
}
