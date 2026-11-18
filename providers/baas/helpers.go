package baas

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
)

// alatDo sends a JSON request to ALAT and decodes the response into T.
// body may be nil (e.g. for GET). Every failure is logged and returned as an
// *appErr.AlatError so the error mapper renders it consistently. Errors ALAT
// didn't send itself have NoResponse set, so callers can tell "ALAT said no"
// from "we don't know".
func alatDo[T any](ctx context.Context, a *Alat, method, url string, body any) (*T, error) {
	if a.BaseURL == "" || a.APIKey == "" {
		log.Printf("[ALAT] level=ERROR %s %s: client not configured (missing ALAT_BASE_URL or ALAT_API_KEY)", method, url)
		return nil, &appErr.AlatError{Message: "wema alat service is not configured", NoResponse: true}
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			log.Printf("[ALAT] level=ERROR %s %s: marshal request failed: %v", method, url, err)
			return nil, &appErr.AlatError{Message: "failed to marshal alat request", NoResponse: true}
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		log.Printf("[ALAT] level=ERROR %s %s: build request failed: %v", method, url, err)
		return nil, &appErr.AlatError{Message: "failed to create alat request", NoResponse: true}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-api-key", a.APIKey)
	if a.SubscriptionKey != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", a.SubscriptionKey)
	}

	// Log the path only - query strings can carry customer data.
	path := req.URL.Path
	started := time.Now()

	resp, err := a.Client.Do(req)
	if err != nil {
		log.Printf("[ALAT] level=ERROR %s %s: transport error after %s (ctx_err=%v): %v",
			method, path, time.Since(started).Round(time.Millisecond), ctx.Err(), err)
		return nil, &appErr.AlatError{Message: "alat request failed", NoResponse: true}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		log.Printf("[ALAT] level=ERROR %s %s status=%d dur=%s: read body failed: %v", method, path, resp.StatusCode, elapsed, err)
		return nil, &appErr.AlatError{Message: "failed to read alat response", NoResponse: true}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ALAT] level=WARN %s %s status=%d dur=%s request_id=%s body=%s",
			method, path, resp.StatusCode, elapsed, alatRequestID(resp.Header), truncateForLog(respBytes))
		// Prefer ALAT's own error envelope when it sends one.
		var alatErr appErr.AlatError
		if json.Unmarshal(respBytes, &alatErr) == nil && (alatErr.Message != "" || alatErr.Code != "" || len(alatErr.Errors) > 0) {
			// A 5xx is ALAT failing, not ALAT answering, even with a body.
			alatErr.NoResponse = resp.StatusCode >= 500
			return nil, &alatErr
		}
		return nil, &appErr.AlatError{
			StatusCode: strconv.Itoa(resp.StatusCode),
			Message:    "alat request returned an unexpected status",
			NoResponse: true,
		}
	}

	var out T
	if len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, &out); err != nil {
			log.Printf("[ALAT] level=ERROR %s %s status=%d dur=%s: decode failed: %v body=%s",
				method, path, resp.StatusCode, elapsed, err, truncateForLog(respBytes))
			return nil, &appErr.AlatError{Message: "failed to decode alat response", NoResponse: true}
		}
	}

	log.Printf("[ALAT] level=INFO %s %s status=%d dur=%s", method, path, resp.StatusCode, elapsed)
	return &out, nil
}

// alatRequestID picks out whichever correlation header the gateway sets, to
// quote to Wema when escalating a failed call.
func alatRequestID(h http.Header) string {
	for _, k := range []string{"X-Request-Id", "X-Correlation-Id", "Request-Id", "Apim-Request-Id"} {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return "-"
}

// truncateForLog caps response bodies in logs. Bodies are only logged on
// error/decode paths, which carry ALAT messages rather than account data.
func truncateForLog(b []byte) string {
	const max = 512
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}
