package wema

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"neat_mobile_app_backend/internal/modules/tier"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
)

func TestSigner(t *testing.T) {
	s := NewSigner("secret")
	sig, err := s.Sign(PurposeSecurityInfo, "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sig, ".") {
		t.Errorf("signature %q contains '.', which statement IDs use as a separator", sig)
	}
	if !s.Verify(PurposeSecurityInfo, "tx-1", sig) {
		t.Error("valid signature rejected")
	}
	if s.Verify(PurposeStatement, "tx-1", sig) {
		t.Error("signature accepted for a different purpose")
	}
	if s.Verify(PurposeSecurityInfo, "tx-2", sig) {
		t.Error("signature accepted for a different value")
	}
	if NewSigner("other").Verify(PurposeSecurityInfo, "tx-1", sig) {
		t.Error("signature accepted under a different secret")
	}
	if s.Verify(PurposeSecurityInfo, "tx-1", "not base64 !!") {
		t.Error("garbage signature accepted")
	}

	empty := NewSigner("  ")
	if empty.Configured() {
		t.Error("blank secret reported as configured")
	}
	if _, err := empty.Sign(PurposeSecurityInfo, "tx-1"); err == nil {
		t.Error("expected Sign to fail without a secret")
	}
	if empty.Verify(PurposeSecurityInfo, "tx-1", sig) {
		t.Error("unconfigured signer verified a signature")
	}
}

func TestNormaliseStatus(t *testing.T) {
	cases := map[string]wallet.ProviderTransferStatus{
		"":             wallet.ProviderTransferPending,
		"Pending":      wallet.ProviderTransferPending,
		"Processing":   wallet.ProviderTransferPending,
		"Successful":   wallet.ProviderTransferSuccessful,
		"SUCCESS":      wallet.ProviderTransferSuccessful,
		"Completed":    wallet.ProviderTransferSuccessful,
		"Failed":       wallet.ProviderTransferFailed,
		"Reversed":     wallet.ProviderTransferFailed,
		"Declined":     wallet.ProviderTransferFailed,
		"Rejected":     wallet.ProviderTransferFailed,
		"Cancelled":    wallet.ProviderTransferFailed,
		"Unsuccessful": wallet.ProviderTransferFailed,
		"Unrecognised": wallet.ProviderTransferPending,
	}
	for in, want := range cases {
		if got := normaliseStatus(in); got != want {
			t.Errorf("normaliseStatus(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParseNaira(t *testing.T) {
	for in, want := range map[string]float64{"1000.50": 1000.5, "1,000.50": 1000.5, "NGN 2,500": 2500, "₦12": 12, " 0 ": 0} {
		got, err := parseNaira(in)
		if err != nil || got != want {
			t.Errorf("parseNaira(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc"} {
		if _, err := parseNaira(in); err == nil {
			t.Errorf("parseNaira(%q): expected an error", in)
		}
	}
}

// fakeALAT serves canned bodies by path and records every request.
type fakeALAT struct {
	t      *testing.T
	bodies map[string]string

	mu    sync.Mutex
	calls []recordedCall
}

type recordedCall struct {
	method, path, query string
	body                map[string]any
}

func (f *fakeALAT) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.calls = append(f.calls, recordedCall{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
	f.mu.Unlock()
	resp, ok := f.bodies[r.URL.Path]
	if !ok {
		f.t.Errorf("unexpected ALAT call %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(resp))
}

func (f *fakeALAT) call(path string) *recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.calls {
		if f.calls[i].path == path {
			return &f.calls[i]
		}
	}
	return nil
}

func newFakeALAT(t *testing.T, bodies map[string]string) (*fakeALAT, *baas.Alat) {
	t.Helper()
	f := &fakeALAT{t: t, bodies: bodies}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, baas.NewAlat(srv.URL, "key", "sub", "channel")
}

const okTransfer = `{"result":{"status":"Pending","message":"Request received","transactionReference":"tx-1","platformTransactionReference":"PLT-1"},"hasError":false}`

func TestTransferAdapter_InitiateTransfer(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		"/debit-wallet/api/Shared/AccountNameEnquiry/Wallet/0123456789": `{"result":{"accountName":"ADA OBI","accountNumber":"0123456789"},"hasError":false}`,
		"/debit-wallet/api/Shared/GetAllBanks":                          `{"result":[{"bankName":"ACCESS BANK","bankCode":"000014"}],"hasError":false}`,
		"/debit-wallet/api/Shared/ProcessClientTransfer":                okTransfer,
	})
	signer := NewSigner("secret")
	a := NewTransferAdapter(alat, signer)

	name, narration := "OBI ADA", "Lunch"
	resp, err := a.InitiateTransfer(context.Background(),
		wallet.TransferSource{AccountNumber: "0123456789", WalletCustomerID: "08030000000"},
		&wallet.TransferRequest{
			Amount:        250050, // kobo
			SortCode:      "000014",
			AccountNumber: "9876543210",
			AccountName:   &name,
			Narration:     &narration,
			Metadata:      map[string]any{"internal_transaction_id": "tx-1"},
		})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Status || resp.Transfer.TransactionReference != "tx-1" || resp.Transfer.Amount != 2500.5 || resp.Transfer.Charges != 0 {
		t.Errorf("unexpected response: %+v", resp)
	}

	c := f.call("/debit-wallet/api/Shared/ProcessClientTransfer")
	if c == nil {
		t.Fatal("ProcessClientTransfer not called")
	}
	want := map[string]any{
		"amount": 2500.5, "destinationBankCode": "000014", "destinationBankName": "ACCESS BANK",
		"destinationAccountNumber": "9876543210", "destinationAccountName": "OBI ADA",
		"sourceAccountNumber": "0123456789", "narration": "Lunch", "transactionReference": "tx-1",
		"useCustomNarration": true,
	}
	for k, v := range want {
		if c.body[k] != v {
			t.Errorf("payload[%s] = %v, want %v", k, c.body[k], v)
		}
	}
	if si, _ := c.body["securityInfo"].(string); !signer.Verify(PurposeSecurityInfo, "tx-1", si) {
		t.Errorf("securityInfo %q does not verify for the reference", si)
	}
	if f.call("/debit-wallet/api/Shared/AccountNameEnquiry/Wallet/0123456789") == nil {
		t.Error("source account enquiry (Step 1) was skipped")
	}
}

func TestTransferAdapter_InitiateTransfer_Guards(t *testing.T) {
	_, alat := newFakeALAT(t, map[string]string{})
	if _, err := NewTransferAdapter(alat, NewSigner("s")).InitiateTransfer(context.Background(), wallet.TransferSource{AccountNumber: "1"}, &wallet.TransferRequest{Amount: 100}); err == nil {
		t.Error("expected an error without internal_transaction_id")
	}

	f, alat := newFakeALAT(t, map[string]string{
		"/debit-wallet/api/Shared/AccountNameEnquiry/Wallet/1": `{"result":{"accountName":"A"},"hasError":false}`,
	})
	_, err := NewTransferAdapter(alat, NewSigner("")).InitiateTransfer(context.Background(), wallet.TransferSource{AccountNumber: "1"},
		&wallet.TransferRequest{Amount: 100, Metadata: map[string]any{"internal_transaction_id": "tx-1"}})
	if err == nil {
		t.Error("expected an error without a signing secret")
	}
	if f.call("/debit-wallet/api/Shared/ProcessClientTransfer") != nil {
		t.Error("transfer submitted unsigned")
	}
}

func TestTransferAdapter_InitiateTransfer_ExplicitFailureIsUnsuccessful(t *testing.T) {
	_, alat := newFakeALAT(t, map[string]string{
		"/debit-wallet/api/Shared/AccountNameEnquiry/Wallet/1": `{"result":{"accountName":"A"},"hasError":false}`,
		"/debit-wallet/api/Shared/GetAllBanks":                 `{"result":[],"hasError":false}`,
		"/debit-wallet/api/Shared/ProcessClientTransfer":       `{"result":{"status":"Failed","message":"Limit exceeded"},"hasError":false}`,
	})
	resp, err := NewTransferAdapter(alat, NewSigner("s")).InitiateTransfer(context.Background(), wallet.TransferSource{AccountNumber: "1"},
		&wallet.TransferRequest{Amount: 100, SortCode: "000014", Metadata: map[string]any{"internal_transaction_id": "tx-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status || resp.Message != "Limit exceeded" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestTransferAdapter_FundWallet(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		"/credit-wallet/api/Shared/AccountNameEnquiry/Wallet/0123456789": `{"result":{"accountName":"ADA OBI"},"hasError":false}`,
		"/credit-wallet/api/IntraBankTransfer/FundWallet":                okTransfer,
	})
	signer := NewSigner("secret")
	resp, err := NewTransferAdapter(alat, signer).FundWallet(context.Background(), "0123456789", 1500, "Cashback", "tx-9")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Status || resp.Transfer.TransactionReference != "tx-9" {
		t.Errorf("unexpected response: %+v", resp)
	}
	c := f.call("/credit-wallet/api/IntraBankTransfer/FundWallet")
	if c == nil || c.body["amount"] != 1500.0 || c.body["transactionReference"] != "tx-9" || c.body["destinationAccountNumber"] != "0123456789" {
		t.Fatalf("unexpected FundWallet payload: %+v", c)
	}
	if si, _ := c.body["securityInfo"].(string); !signer.Verify(PurposeSecurityInfo, "tx-9", si) {
		t.Error("securityInfo does not verify")
	}
}

func TestTransferAdapter_StatusChecks(t *testing.T) {
	_, alat := newFakeALAT(t, map[string]string{
		"/debit-wallet/api/IntraBankTransfer/ConfirmClientTransferStatus/tx-1":  `{"result":{"data":{"status":"Successful"}},"hasError":false}`,
		"/credit-wallet/api/IntraBankTransfer/ConfirmClientTransferStatus/tx-2": `{"result":{"data":{"status":"Failed"}},"hasError":false}`,
		"/debit-wallet/api/IntraBankTransfer/ConfirmClientTransferStatus/tx-3":  `{"result":{"data":null},"hasError":false}`,
	})
	a := NewTransferAdapter(alat, NewSigner("s"))
	ctx := context.Background()
	if got, err := a.CheckDebitStatus(ctx, "tx-1"); err != nil || got != wallet.ProviderTransferSuccessful {
		t.Errorf("debit tx-1 = %s, %v", got, err)
	}
	if got, err := a.CheckCreditStatus(ctx, "tx-2"); err != nil || got != wallet.ProviderTransferFailed {
		t.Errorf("credit tx-2 = %s, %v", got, err)
	}
	if got, err := a.CheckDebitStatus(ctx, "tx-3"); err != nil || got != wallet.ProviderTransferPending {
		t.Errorf("debit tx-3 (no record yet) = %s, %v", got, err)
	}
}

func TestTransferAdapter_BalanceAndBanks(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		"/ws-acct-mgt/api/AccountMaintenance/CustomerAccount/GetAccountV2/accountNumber/0123456789": `{"result":{"walletNumber":"0123456789","availableBalance":"1,250.75"},"successful":true}`,
		"/debit-wallet/api/Shared/GetAllBanks":                                                      `{"result":{"bankName":"ACCESS BANK","bankCode":"000014"},"hasError":false}`,
	})
	a := NewTransferAdapter(alat, NewSigner("s"))
	ctx := context.Background()

	d, err := a.GetCustomerDetails(ctx, "0123456789")
	if err != nil || d.Customer.AvailableBalance != 1250.75 || d.Customer.AccountNumber != "0123456789" {
		t.Errorf("GetCustomerDetails = %+v, %v", d, err)
	}
	ad, err := NewAccountFinder(alat).GetCustomerDetails(ctx, "0123456789")
	if err != nil || ad.Customer.AvailableBalance != 1250.75 {
		t.Errorf("AccountFinder.GetCustomerDetails = %+v, %v", ad, err)
	}

	// The documented single-object bank shape still yields a list, and the
	// list is cached.
	for i := 0; i < 2; i++ {
		banks, err := a.FetchBanks(ctx)
		if err != nil || len(banks) != 1 || banks[0].Code != "000014" {
			t.Fatalf("FetchBanks = %+v, %v", banks, err)
		}
	}
	n := 0
	for _, c := range f.calls {
		if c.path == "/debit-wallet/api/Shared/GetAllBanks" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("GetAllBanks called %d times, want 1 (cached)", n)
	}
}

func TestTierUpgrader(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		"/account-upgrade/api/partnership/partner-account-upgrade-tier2": `{"status":true,"message":"ok"}`,
		"/account-upgrade/api/partnership/partner-account-upgrade-tier3": `{"status":false,"message":"Address not verified","code":"05"}`,
	})
	u := NewTierUpgrader(alat)
	if err := u.UpgradeToTier2(context.Background(), "0123", "nin", "bvn", "img"); err != nil {
		t.Fatal(err)
	}
	c := f.call("/account-upgrade/api/partnership/partner-account-upgrade-tier2")
	if c == nil || c.body["accountNumber"] != "0123" || c.body["nin"] != "nin" || c.body["bvn"] != "bvn" || c.body["liveImageOfFace"] != "img" {
		t.Errorf("unexpected tier2 payload: %+v", c)
	}
	err := u.UpgradeToTier3(context.Background(), "0123", tier.ResidentialAddress{Street: "Broad St", PostalCode: "100001"})
	if err == nil || !strings.Contains(err.Error(), "Address not verified") {
		t.Errorf("tier3 err = %v, want the ALAT message", err)
	}
	c = f.call("/account-upgrade/api/partnership/partner-account-upgrade-tier3")
	addr, _ := c.body["residentialAddress"].(map[string]any)
	if addr["street"] != "Broad St" || addr["postalCode"] != "100001" {
		t.Errorf("unexpected tier3 address: %+v", addr)
	}
}
