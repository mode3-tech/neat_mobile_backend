package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		return errors.New("sms service not configured")
	}

	url := "https://api.smslive247.com/api/v4/sms"

	payload := map[string]string{
		"senderID":     s.senderID,
		"mobileNumber": destination,
		"messageText":  "Your OTP is 123456. It expires in 5 minutes.",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(respBody) == 0 {
			return fmt.Errorf("sms send failed with status: %d", resp.StatusCode)
		}
		return fmt.Errorf("sms send failed with status: %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}
