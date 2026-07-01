package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

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
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", z.Url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", z.ApiKey)

	resp, err := z.Client.Do(req)
	if err != nil {
		log.Printf("failed to send request: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("unexpected status code: %d (failed to read response body: %v)", resp.StatusCode, readErr)
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		log.Printf("unexpected status code: %d, response: %s", resp.StatusCode, respBody)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, respBody)
	}

	log.Printf("email sent successfully")
	return nil
}
