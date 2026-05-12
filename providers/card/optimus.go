package card

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Optimus struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOptimus(baseURL, apiKey string, client *http.Client) *Optimus {
	return &Optimus{baseURL: baseURL, apiKey: apiKey, client: &http.Client{Timeout: time.Second * 15}}
}

func (o *Optimus) RequestCard(ctx context.Context, requestInfo *OptimusCardRequest) error {
	baseURL := strings.TrimSpace(o.baseURL)
	apiKey := strings.TrimSpace(o.apiKey)

	if baseURL == "" || apiKey == "" {
		log.Println("Optimus card service is not configured")
		return errors.New("Optimus card service is not configured")
	}

	url := baseURL + "/Card/request"

	body, err := json.Marshal(&requestInfo)
	if err != nil {
		log.Printf("Failed to decode card request payload: %s\n", err)
		return fmt.Errorf("Failed to decode card request payload: %s\n", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to make request for card: %s\n", err)
		return fmt.Errorf("Failed to make request for card: %s\n", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/plain")

	resp, err := o.client.Do(req)
	if err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			if len(respBody) == 0 {
				log.Printf("Optimus card request failed with status: %d", resp.StatusCode)
				return fmt.Errorf("Optimus card request failed with status: %d", resp.StatusCode)
			}
			log.Printf("Optimus card request failed: %s", err)
			return fmt.Errorf("Optimus card request failed: %s", err)
		}
	}

	var result OptimusCardResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to map json to result: %s\n", err)
		return fmt.Errorf("Failed to map json to result: %s\n", err)
	}
	return nil
}
