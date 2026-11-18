package vas

import (
	"testing"

	walletmodule "neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
)

func TestWemaVASStatusIsConservative(t *testing.T) {
	tests := map[string]walletmodule.ProviderTransferStatus{
		"Successful":   walletmodule.ProviderTransferSuccessful,
		"Completed":    walletmodule.ProviderTransferSuccessful,
		"Unsuccessful": walletmodule.ProviderTransferFailed,
		"Failed":       walletmodule.ProviderTransferFailed,
		"Pending":      walletmodule.ProviderTransferPending,
		"Unknown":      walletmodule.ProviderTransferPending,
	}
	for status, want := range tests {
		got, _ := wemaVASStatus(&baas.AlatClientVASResponse{
			Result: &baas.AlatClientVASTransaction{Status: status},
		})
		if got != want {
			t.Errorf("wemaVASStatus(%q) = %s, want %s", status, got, want)
		}
	}
}

func TestWemaAirtimePendingResultIncludesStatusLookupID(t *testing.T) {
	result := wemaAirtimeResult("local-tx-id", "", "Pending ALAT confirmation", "MTN", "08000000000", 100, false)
	if result.ResponseCode != "01" || result.RequestID != "local-tx-id" {
		t.Fatalf("pending result = %+v, want pending code and local transaction ID", result)
	}
}

func TestWemaDataSelection(t *testing.T) {
	s := &Service{}
	for _, test := range []struct {
		payload DataPayload
		network string
		code    int
		wantErr bool
	}{
		{payload: DataPayload{UniqueCode: "MTN|17"}, network: "MTN", code: 17},
		{payload: DataPayload{Network: "Airtel", PackageCode: 23}, network: "Airtel", code: 23},
		{payload: DataPayload{UniqueCode: "not-a-package"}, wantErr: true},
	} {
		network, code, err := s.wemaDataSelection(test.payload)
		if (err != nil) != test.wantErr || network != test.network || code != test.code {
			t.Errorf("wemaDataSelection(%+v) = %q, %d, %v", test.payload, network, code, err)
		}
	}
}
