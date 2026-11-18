package wallet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neat_mobile_app_backend/internal/modules/transaction"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func newAlatWebhookRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAlatWebhookRoutes(r.Group("/api/v1"), &Handler{}, passThrough)
	return r
}

func TestHandleAlatAccountCreation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"documented payload", `{"title":"t","message":"m","data":{"email":"user@alat.ng","nuban":"0123456789","nubanName":"JANE DOE","type":1,"nubanStatus":"Active","phoneNumber":"07011223344"},"requestType":2}`, http.StatusOK},
		{"no data", `{"title":"t","message":"m","requestType":2}`, http.StatusOK},
		{"invalid json", `{`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/alat/account/creation", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			newAlatWebhookRouter().ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func pendingTxRow() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "wallet_id", "provider_reference", "type", "status", "amount"}).
		AddRow("tx-1", "iw-1", "tx-1", string(transaction.TransactionTypeDebit), string(transaction.TransactionStatusPending), 50000)
}

const findPendingWemaTxPattern = `SELECT wallet_transactions\.\* FROM "wallet_transactions" JOIN wallet_customer_wallets .* WHERE wallet_transactions\.id = \$1 AND LOWER\(wallet_customer_wallets\.provider\) IN \(\$2,\$3\) AND wallet_transactions\.status = \$4 AND wallet_transactions\.provider_reference <> ''`

func TestHandleAlatTransactionCallback(t *testing.T) {
	t.Run("pending wema debit is settled after confirming with alat", func(t *testing.T) {
		wema := &fakeWema{debitStatus: ProviderTransferSuccessful}
		svc, mock, cleanup := newWemaTestService(t, wema)
		defer cleanup()
		mock.ExpectQuery(findPendingWemaTxPattern).WithArgs("tx-1", ProviderWema, ProviderAlat, transaction.TransactionStatusPending, 1).
			WillReturnRows(pendingTxRow())
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "wallet_transactions" SET "status"=\$1,"updated_at"=\$2 WHERE id = \$3 AND status = \$4`).
			WithArgs(transaction.TransactionStatusSuccessful, sqlmock.AnyArg(), "tx-1", transaction.TransactionStatusPending).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		svc.HandleAlatTransactionCallback(context.Background(), " tx-1 ")

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		if len(wema.checked) != 1 || wema.checked[0] != "debit:tx-1" {
			t.Errorf("checked = %v, want [debit:tx-1]", wema.checked)
		}
	})

	t.Run("unknown or settled reference is ignored without calling alat", func(t *testing.T) {
		wema := &fakeWema{debitStatus: ProviderTransferSuccessful}
		svc, mock, cleanup := newWemaTestService(t, wema)
		defer cleanup()
		mock.ExpectQuery(findPendingWemaTxPattern).WillReturnRows(sqlmock.NewRows([]string{"id"}))

		svc.HandleAlatTransactionCallback(context.Background(), "tx-unknown")

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		if len(wema.checked) != 0 {
			t.Errorf("checked = %v, want no alat calls", wema.checked)
		}
	})

	t.Run("empty reference is ignored", func(t *testing.T) {
		wema := &fakeWema{}
		svc, mock, cleanup := newWemaTestService(t, wema)
		defer cleanup()
		svc.HandleAlatTransactionCallback(context.Background(), "  ")
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		if len(wema.checked) != 0 {
			t.Errorf("checked = %v, want no alat calls", wema.checked)
		}
	})
}

func TestHandleAlatTransaction_Route(t *testing.T) {
	wema := &fakeWema{debitStatus: ProviderTransferSuccessful}
	svc, mock, cleanup := newWemaTestService(t, wema)
	defer cleanup()
	mock.ExpectQuery(findPendingWemaTxPattern).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAlatWebhookRoutes(r.Group("/api/v1"), &Handler{service: svc}, passThrough)

	body := `{"title":"string","message":"string","data":{"status":"Successful","message":"string","narration":"string","transactionReference":"tx-1","platformTransactionReference":"PLT-1","transactionStan":"123","orinalTxnTransactionDate":"0001-01-01T00:00:00"},"request":3,"requestType":3}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alat/transaction", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAlatWebhookRoutes_AuthAndNotification(t *testing.T) {
	svc, _, cleanup := newWemaTestService(t, &fakeWema{})
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAlatWebhookRoutes(r.Group("/api/v1"), &Handler{service: svc}, passThrough)

	for _, tc := range []struct{ path, body string }{
		// Empty reference: rejected before any DB lookup, still answered 200.
		{"/api/v1/alat/auth", `{"transactionReference":"","securityInfo":"x"}`},
		// Documented Transaction Notification model.
		{"/api/v1/alat/transaction/notification", `{"accountNumber":"0123456789","transactionType":"Credit","amount":10000,"narration":"Custom narration","transactionDate":"1990-07-09T08:34:37.504Z"}`},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (body %s)", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestHandleAlatTransactionNotification_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAlatWebhookRoutes(r.Group("/api/v1"), &Handler{}, passThrough)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alat/transaction/notification", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
}

// passThrough stands in for the IP allowlist (tested in internal/middleware).
func passThrough(c *gin.Context) { c.Next() }

type recordingAccountListener struct{ phones chan string }

func (l *recordingAccountListener) OnAlatAccountCreated(ctx context.Context, phone string) {
	l.phones <- phone
}

// The account creation webhook hands the phone to the registration listener.
func TestHandleAlatAccountCreation_NotifiesListener(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := &recordingAccountListener{phones: make(chan string, 1)}
	h := &Handler{}
	h.SetAlatAccountCreationListener(l)
	r := gin.New()
	RegisterAlatWebhookRoutes(r.Group("/api/v1"), h, passThrough)

	body := `{"title":"t","message":"m","data":{"email":"user@alat.ng","nuban":"0123456789","nubanName":"JANE DOE","type":1,"nubanStatus":"Active","phoneNumber":"07011223344"},"requestType":2}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alat/account/creation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	select {
	case phone := <-l.phones:
		if phone != "07011223344" {
			t.Fatalf("listener got %q", phone)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener was not called")
	}
}
