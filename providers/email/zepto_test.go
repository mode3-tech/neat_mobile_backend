package mail

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
)

func TestZeptoSend_MapsProviderError(t *testing.T) {
	const body = `{
		"error": {
			"code": "TM_3201",
			"message": "Invalid data. Mandatory keys missing.",
			"details": [
				{ "code": "GE_102", "message": "This field is required.", "target": "subject" }
			]
		},
		"request_id": "req-123"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	z := NewZepto("key", srv.URL, "from@example.com")

	err := z.Send(context.Background(), []string{"to@example.com"}, "subject", "<p>hi</p>")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var zeptoErr *appErr.ZeptoError
	if !errors.As(err, &zeptoErr) {
		t.Fatalf("expected *appErr.ZeptoError, got %T (%v)", err, err)
	}

	if zeptoErr.Status != http.StatusBadRequest {
		t.Errorf("Status: got %d, want %d", zeptoErr.Status, http.StatusBadRequest)
	}
	if zeptoErr.Code != "TM_3201" {
		t.Errorf("Code: got %q, want %q", zeptoErr.Code, "TM_3201")
	}
	if zeptoErr.SubCode != "GE_102" {
		t.Errorf("SubCode: got %q, want %q", zeptoErr.SubCode, "GE_102")
	}
	if zeptoErr.Target != "subject" {
		t.Errorf("Target: got %q, want %q", zeptoErr.Target, "subject")
	}
	if zeptoErr.Message != "This field is required." {
		t.Errorf("Message: got %q, want %q", zeptoErr.Message, "This field is required.")
	}
	if zeptoErr.RequestID != "req-123" {
		t.Errorf("RequestID: got %q, want %q", zeptoErr.RequestID, "req-123")
	}
}

func TestZeptoSend_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	z := NewZepto("key", srv.URL, "from@example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := z.Send(ctx, []string{"to@example.com"}, "subject", "<p>hi</p>")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}

	var zeptoErr *appErr.ZeptoError
	if !errors.As(err, &zeptoErr) {
		t.Fatalf("expected *appErr.ZeptoError, got %T (%v)", err, err)
	}
	if zeptoErr.Code != "TIMEOUT" {
		t.Errorf("Code: got %q, want %q", zeptoErr.Code, "TIMEOUT")
	}
}
