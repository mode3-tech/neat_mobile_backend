package prembly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	req.Header.Set("x-api-key", strings.TrimSpace(p.apiKey))

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
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("prembly bvn validation failed and response body could not be read: %v", readErr)
		} else if body := strings.TrimSpace(string(bodyBytes)); body != "" {
			log.Printf("prembly bvn validation failed body=%s", body)
		}
		log.Printf("prembly_bvn non-2xx status=%d duration=%s", resp.StatusCode, duration)
		return nil, fmt.Errorf("prembly bvn validation failed with status %d", resp.StatusCode)
	}

	var result bvn.PremblyBVNValidationSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Prembly) ValidateBVNWithFace(ctx context.Context, number, image string) (*bvn.PremblyBVNWithFaceResponse, error) {
	if p.apiKey == "" {
		log.Printf("can't validate bvn with face because api key is missing")
		return nil, fmt.Errorf("api key for bvn validation with face is missing")
	}
	url := "https://api.prembly.com/verification/bvn_w_face"

	payload := map[string]string{
		"number": number,
		"image":  image,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("error occured while marshaling payload: %v", err)
		return nil, fmt.Errorf("error occured while marshaling payload: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("an error occured: %v", err)
		return nil, fmt.Errorf("an error occured: %v", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Printf("request failed: %v", err)
		return nil, fmt.Errorf("request failed: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("prembly bvn validation failed and response body could not be read: %v", readErr)
		} else if body := strings.TrimSpace(string(bodyBytes)); body != "" {
			log.Printf("prembly bvn validation failed body=%s", body)
		}
		log.Printf("prembly_bvn non-2xx status=%d", resp.StatusCode)
		return nil, fmt.Errorf("prembly bvn validation failed with status %d", resp.StatusCode)
	}

	var result bvn.PremblyBVNWithFaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("failed to decode response body: %v", err)
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &result, nil
}
