package mail

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

// zeptoErrorResponse models ZeptoMail's official error JSON structure.
type zeptoErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Target  string `json:"target"`
		} `json:"details"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

type Zepto struct {
	From   string
	ApiKey string
	Url    string
	Client *http.Client
}

func NewZepto(apiKey, url, from string) *Zepto {
	return &Zepto{
		ApiKey: apiKey,
		Url:    url,
		From:   from,
		Client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (z *Zepto) Send(ctx context.Context, to []string, subject, body string) error {
	recipients := make([]map[string]interface{}, 0, len(to))
	for _, addr := range to {
		recipients = append(recipients, map[string]interface{}{
			"email_address": map[string]string{
				"address": addr,
			},
		})
	}

	payload := map[string]interface{}{
		"from": map[string]string{
			"name":    "Neat MC",
			"address": z.From,
		},
		"to":       recipients,
		"subject":  subject,
		"htmlbody": body,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %v", err)
		return &appErr.ZeptoError{Status: 500, Code: "CLIENT_ERROR", Message: "failed to marshal email payload"}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", z.Url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		return &appErr.ZeptoError{Status: 500, Code: "CLIENT_ERROR", Message: "failed to create email request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", z.ApiKey)

	resp, err := z.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Printf("email provider request timed out: %v", err)
			return &appErr.ZeptoError{Status: 408, Code: "TIMEOUT", Message: "email provider request timed out"}
		}
		log.Printf("failed to send request: %v", err)
		return &appErr.ZeptoError{Status: 502, Code: "NETWORK_ERROR", Message: "email provider request failed"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		bodyStr := strings.TrimSpace(string(respBody))
		log.Printf("unexpected status code: %d, response: %s", resp.StatusCode, bodyStr)

		zeptoErr := &appErr.ZeptoError{Status: resp.StatusCode, Message: bodyStr}
		var er zeptoErrorResponse
		if json.Unmarshal(respBody, &er) == nil {
			zeptoErr.Code = er.Error.Code
			zeptoErr.RequestID = er.RequestID
			if len(er.Error.Details) > 0 {
				zeptoErr.SubCode = er.Error.Details[0].Code
				zeptoErr.Message = er.Error.Details[0].Message
				zeptoErr.Target = er.Error.Details[0].Target
			} else if er.Error.Message != "" {
				zeptoErr.Message = er.Error.Message
			}
		}
		if zeptoErr.Message == "" {
			zeptoErr.Message = "email provider returned an unexpected error"
		}
		return zeptoErr
	}

	log.Printf("email sent successfully")
	return nil
}
