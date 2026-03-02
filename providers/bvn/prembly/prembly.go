package prembly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neat_mobile_app_backend/providers/bvn"
	"net/http"
	"strings"
	"time"
)

type Prembly struct {
	apiKey     string
	httpClient *http.Client
}

func NewPrembly(apiKey string) *Prembly {
	return &Prembly{apiKey: apiKey, httpClient: &http.Client{
		Timeout: 10 * time.Second,
	}}
}

func (p *Prembly) ValidateBVNWithPrembly(ctx context.Context, BVN string) (*bvn.PremblyBVNValidationSuccessResponse, error) {
	url := "https://api.prembly.com/verification/bvn"

	payload := map[string]string{
		"number": BVN,
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
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.apiKey))
	fmt.Println("Bearer " + p.apiKey)

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		log.Printf("prembly_bvn request failed duration=%s err=%v", duration, err)
		return nil, err
	}

	log.Printf("prembly_bvn request completed status=%d duration=%s", resp.StatusCode, duration)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("prembly bvn validation failed and response body could not be decoded: %v", err)
		} else {
			log.Printf("prembly bvn validation failed: %s", result)
			log.Printf("prembly_bvn non-2xx status=%d duration=%s", resp.StatusCode, duration)
			return nil, fmt.Errorf("prembly bvn validation failed with status %d", resp.StatusCode)
		}
	}

	var result bvn.PremblyBVNValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
