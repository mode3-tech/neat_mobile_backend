package registerv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"neat_mobile_app_backend/internal/modules/auth"
)

// fakeWalletService is a minimal auth.WalletService test double - only the
// methods recoverWalletByPhone actually calls need real behavior.
type fakeWalletService struct {
	lookupCustomerByPhoneCalls   int
	lookupCustomerByPhoneFunc    func(ctx context.Context, phone string) (string, bool, error)
	lookupWalletByCustomerIDFunc func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error)
}

func (f *fakeWalletService) GenerateWallet(ctx context.Context, payload *auth.WalletPayload) (*auth.WalletResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWalletService) LookupWalletByCustomerID(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
	return f.lookupWalletByCustomerIDFunc(ctx, customerID)
}

func (f *fakeWalletService) LookupCustomerByPhone(ctx context.Context, phone string) (string, bool, error) {
	f.lookupCustomerByPhoneCalls++
	return f.lookupCustomerByPhoneFunc(ctx, phone)
}

func withNoBackoffDelay(t *testing.T) {
	t.Helper()
	original := recoverWalletByPhoneBackoff
	recoverWalletByPhoneBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { recoverWalletByPhoneBackoff = original })
}

func TestRecoverWalletByPhone_FoundImmediately(t *testing.T) {
	withNoBackoffDelay(t)

	wantResp := &auth.WalletResponse{Customer: &auth.WalletCustomer{ID: "customer-1"}, Wallet: &auth.WalletInfo{WalletId: "wallet-1"}}
	wallet := &fakeWalletService{
		lookupCustomerByPhoneFunc: func(ctx context.Context, phone string) (string, bool, error) {
			if phone != "2348030223346" {
				t.Fatalf("unexpected phone: %q", phone)
			}
			return "customer-1", true, nil
		},
		lookupWalletByCustomerIDFunc: func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
			if customerID != "customer-1" {
				t.Fatalf("unexpected customer id: %q", customerID)
			}
			return wantResp, true, nil
		},
	}

	s := &Service{}
	got := s.recoverWalletByPhone(context.Background(), wallet, "2348030223346")
	if got != wantResp {
		t.Fatalf("expected recovered wallet response, got %+v", got)
	}
	if wallet.lookupCustomerByPhoneCalls != 1 {
		t.Fatalf("expected exactly 1 phone lookup call, got %d", wallet.lookupCustomerByPhoneCalls)
	}
}

func TestRecoverWalletByPhone_FoundOnRetry(t *testing.T) {
	withNoBackoffDelay(t)

	wantResp := &auth.WalletResponse{Customer: &auth.WalletCustomer{ID: "customer-1"}, Wallet: &auth.WalletInfo{WalletId: "wallet-1"}}
	attempt := 0
	wallet := &fakeWalletService{
		lookupCustomerByPhoneFunc: func(ctx context.Context, phone string) (string, bool, error) {
			attempt++
			if attempt < 3 {
				return "", false, nil
			}
			return "customer-1", true, nil
		},
		lookupWalletByCustomerIDFunc: func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
			return wantResp, true, nil
		},
	}

	s := &Service{}
	got := s.recoverWalletByPhone(context.Background(), wallet, "2348030223346")
	if got != wantResp {
		t.Fatalf("expected recovered wallet response on retry, got %+v", got)
	}
	if wallet.lookupCustomerByPhoneCalls != 3 {
		t.Fatalf("expected 3 phone lookup attempts, got %d", wallet.lookupCustomerByPhoneCalls)
	}
}

func TestRecoverWalletByPhone_NeverFound(t *testing.T) {
	withNoBackoffDelay(t)

	wallet := &fakeWalletService{
		lookupCustomerByPhoneFunc: func(ctx context.Context, phone string) (string, bool, error) {
			return "", false, nil
		},
		lookupWalletByCustomerIDFunc: func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
			t.Fatal("should not look up wallet by customer id when phone lookup never finds a customer")
			return nil, false, nil
		},
	}

	s := &Service{}
	got := s.recoverWalletByPhone(context.Background(), wallet, "2348030223346")
	if got != nil {
		t.Fatalf("expected nil when nothing is recoverable, got %+v", got)
	}
	if wallet.lookupCustomerByPhoneCalls != len(recoverWalletByPhoneBackoff) {
		t.Fatalf("expected %d phone lookup attempts, got %d", len(recoverWalletByPhoneBackoff), wallet.lookupCustomerByPhoneCalls)
	}
}

func TestRecoverWalletByPhone_WalletLookupNilResponseIsTreatedAsNotRecovered(t *testing.T) {
	withNoBackoffDelay(t)

	// Mirrors the real Optimus stub behavior (found=true, response=nil) -
	// recoverWalletByPhone must not treat that as a successful recovery.
	wallet := &fakeWalletService{
		lookupCustomerByPhoneFunc: func(ctx context.Context, phone string) (string, bool, error) {
			return "customer-1", true, nil
		},
		lookupWalletByCustomerIDFunc: func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
			return nil, true, nil
		},
	}

	s := &Service{}
	got := s.recoverWalletByPhone(context.Background(), wallet, "2348030223346")
	if got != nil {
		t.Fatalf("expected nil when wallet lookup returns a nil response, got %+v", got)
	}
}

func TestRecoverWalletByPhone_ContextCancelledStopsRetrying(t *testing.T) {
	// Uses the real (non-zero) backoff so the context has time to expire
	// between attempts.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	wallet := &fakeWalletService{
		lookupCustomerByPhoneFunc: func(ctx context.Context, phone string) (string, bool, error) {
			return "", false, nil
		},
		lookupWalletByCustomerIDFunc: func(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
			return nil, false, nil
		},
	}

	s := &Service{}
	got := s.recoverWalletByPhone(ctx, wallet, "2348030223346")
	if got != nil {
		t.Fatalf("expected nil after context cancellation, got %+v", got)
	}
	if wallet.lookupCustomerByPhoneCalls >= len(recoverWalletByPhoneBackoff) {
		t.Fatalf("expected context cancellation to stop retries before exhausting the backoff schedule, got %d calls", wallet.lookupCustomerByPhoneCalls)
	}
}
