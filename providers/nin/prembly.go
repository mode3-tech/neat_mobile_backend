package nin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type NIN struct {
	apiKey     string
	httpClient *http.Client
}

func NewNIN(apiKey string) *NIN {
	return &NIN{apiKey: apiKey, httpClient: &http.Client{
		Timeout: 10 * time.Second,
	}}
}

func (n *NIN) ValidateNIN(ctx context.Context, nin string) (bool, error) {
	url := "https://api.prembly.com/verification/vnin"

	payload := map[string]string{
		"number_nin": nin,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(n.apiKey))

	start := time.Now()
	resp, err := n.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		log.Printf("prembly_nin request failed duration=%s err=%v", duration, err)
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("prembly nin validation failed and response body could not be decoded: %v", err)
		} else {
			log.Printf("prembly nin validation failed: %s", result)
			log.Printf("prembly_nin non-2xx status=%d duration=%s", resp.StatusCode, duration)
			return false, fmt.Errorf("prembly nin validation failed with status %d", resp.StatusCode)
		}
	}

	return true, nil
}
