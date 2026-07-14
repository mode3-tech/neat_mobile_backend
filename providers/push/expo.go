package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neat_mobile_app_backend/internal/modules/notification"
	"net/http"
	"strings"
	"time"
)

const maxExpoPushBatchSize = 100

const expoTooManyExperienceIDsCode = "PUSH_TOO_MANY_EXPERIENCE_IDS"

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

// expoErrorBody mirrors Expo's push API error envelope so specific error
// codes can be detected instead of treating every non-2xx as fatal.
type expoErrorBody struct {
	Errors []struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Details map[string][]string `json:"details"`
	} `json:"errors"`
}

// tooManyExperienceIDsError means a single request mixed push tokens
// belonging to more than one Expo project. TokensByProject maps each
// project's "@owner/slug" identifier to the tokens Expo attributed to it.
type tooManyExperienceIDsError struct {
	TokensByProject map[string][]string
}

func (e *tooManyExperienceIDsError) Error() string {
	return "expo push: request mixed tokens from multiple projects"
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

		tickets, err := c.sendChunk(ctx, messages[start:end])
		if err != nil {
			return nil, err
		}
		allTickets = append(allTickets, tickets...)
	}

	return allTickets, nil
}

// sendChunk sends a single batch (already within Expo's max batch size). If
// Expo rejects the batch because it mixes tokens from more than one project,
// it splits the batch by project and retries each group separately, then
// reassembles tickets in the original message order so callers can still
// correlate tickets[i] with messages[i]. A token whose project-specific
// retry fails gets a zero-value ticket rather than failing the whole chunk.
func (c *ExpoClient) sendChunk(ctx context.Context, messages []notification.ExpoPushMessage) ([]notification.ExpoPushTicket, error) {
	tickets, conflict, err := c.postPush(ctx, messages)
	if err != nil {
		return nil, err
	}
	if conflict == nil {
		return tickets, nil
	}

	log.Printf("expo push: request mixed tokens from %d projects, splitting and retrying", len(conflict.TokensByProject))

	ticketByToken := make(map[string]notification.ExpoPushTicket, len(messages))
	for project, tokens := range conflict.TokensByProject {
		tokenSet := make(map[string]bool, len(tokens))
		for _, t := range tokens {
			tokenSet[t] = true
		}

		group := make([]notification.ExpoPushMessage, 0, len(tokens))
		for _, m := range messages {
			if tokenSet[m.To] {
				group = append(group, m)
			}
		}
		if len(group) == 0 {
			continue
		}

		groupTickets, _, err := c.postPush(ctx, group)
		if err != nil {
			log.Printf("expo push: retry for project %s failed: %v", project, err)
			continue
		}
		for i, m := range group {
			if i < len(groupTickets) {
				ticketByToken[m.To] = groupTickets[i]
			}
		}
	}

	result := make([]notification.ExpoPushTicket, len(messages))
	for i, m := range messages {
		result[i] = ticketByToken[m.To]
	}
	return result, nil
}

// postPush issues a single push/send request. On success it returns the
// tickets. On a PUSH_TOO_MANY_EXPERIENCE_IDS rejection it returns a typed
// conflict instead of a plain error so the caller can recover; any other
// failure is returned as a plain error.
func (c *ExpoClient) postPush(ctx context.Context, messages []notification.ExpoPushMessage) ([]notification.ExpoPushTicket, *tooManyExperienceIDsError, error) {
	status, payload, err := c.rawRequest(ctx, http.MethodPost, "/--/api/v2/push/send", messages)
	if err != nil {
		return nil, nil, err
	}

	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		var errBody expoErrorBody
		if json.Unmarshal(payload, &errBody) == nil {
			for _, e := range errBody.Errors {
				if e.Code == expoTooManyExperienceIDsCode && len(e.Details) > 0 {
					return nil, &tooManyExperienceIDsError{TokensByProject: e.Details}, nil
				}
			}
		}
		return nil, nil, fmt.Errorf("expo push service returned %d: %s", status, strings.TrimSpace(string(payload)))
	}

	var response struct {
		Data []notification.ExpoPushTicket `json:"data"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &response); err != nil {
			return nil, nil, fmt.Errorf("decode expo response: %w", err)
		}
	}

	return response.Data, nil, nil
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
	status, payload, err := c.rawRequest(ctx, method, path, reqBody)
	if err != nil {
		return err
	}

	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("expo push service returned %d: %s", status, strings.TrimSpace(string(payload)))
	}

	if respBody == nil || len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, respBody); err != nil {
		return fmt.Errorf("decode expo response: %w", err)
	}

	return nil
}

func (c *ExpoClient) rawRequest(ctx context.Context, method, path string, reqBody any) (int, []byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal expo request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("build expo request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("send expo request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read expo response: %w", err)
	}

	return resp.StatusCode, payload, nil
}
