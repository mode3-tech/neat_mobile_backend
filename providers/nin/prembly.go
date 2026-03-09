package nin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (n *NIN) ValidateNIN(ctx context.Context, nin string) (*PremblyNINValidationSuccessResponse, error) {
	url := "https://api.prembly.com/verification/vnin"

	payload := map[string]string{
		"number_nin": nin,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("x-api-key", strings.TrimSpace(n.apiKey))

	start := time.Now()
	resp, err := n.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		log.Printf("prembly_nin request failed duration=%s err=%v", duration, err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("prembly nin validation failed and response body could not be read: %v", readErr)
		} else if body := strings.TrimSpace(string(bodyBytes)); body != "" {
			log.Printf("prembly nin validation failed body=%s", body)
		}
		log.Printf("prembly_nin non-2xx status=%d duration=%s", resp.StatusCode, duration)
		return nil, fmt.Errorf("prembly nin validation failed with status %d", resp.StatusCode)
	}

	var result PremblyNINValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("prembly nin validation response body could not be decoded: %v", err)
		return nil, err
	}

	return &result, nil
}
