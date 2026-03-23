package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"neat_mobile_app_backend/modules/notification"
	"net/http"
	"strings"
	"time"
)

const maxExpoPushBatchSize = 100

type ExpoClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

func NewExpoClient(baseURL, accessToken string) *ExpoClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://exp.host"
	}

	return &ExpoClient{
		baseURL:     baseURL,
		accessToken: strings.TrimSpace(accessToken),
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *ExpoClient) Send(ctx context.Context, messages []notification.ExpoPushMessage) ([]notification.ExpoPushTicket, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	allTickets := make([]notification.ExpoPushTicket, 0, len(messages))
	for start := 0; start < len(messages); start += maxExpoPushBatchSize {
		end := start + maxExpoPushBatchSize
		if end > len(messages) {
			end = len(messages)
		}

		var response struct {
			Data []notification.ExpoPushTicket `json:"data"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/--/api/v2/push/send", messages[start:end], &response); err != nil {
			return nil, err
		}

		allTickets = append(allTickets, response.Data...)
	}

	return allTickets, nil
}

func (c *ExpoClient) GetReceipts(ctx context.Context, receiptIDs []string) (map[string]notification.ExpoPushReceipt, error) {
	if len(receiptIDs) == 0 {
		return map[string]notification.ExpoPushReceipt{}, nil
	}

	var response struct {
		Data map[string]notification.ExpoPushReceipt `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/--/api/v2/push/getReceipts", map[string]any{
		"ids": receiptIDs,
	}, &response); err != nil {
		return nil, err
	}

	if response.Data == nil {
		return map[string]notification.ExpoPushReceipt{}, nil
	}

	return response.Data, nil
}

func (c *ExpoClient) doJSON(ctx context.Context, method, path string, reqBody any, respBody any) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal expo request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build expo request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send expo request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read expo response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("expo push service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	if respBody == nil || len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, respBody); err != nil {
		return fmt.Errorf("decode expo response: %w", err)
	}

	return nil
}
