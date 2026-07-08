package auth

import "testing"

func TestWalletPhoneFromSnapshot(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *registrationJobSnapshot
		want     string
	}{
		{
			name:     "prefers wallet phone (BVN phone) over reachable phone",
			snapshot: &registrationJobSnapshot{Phone: "2348010000000", WalletPhone: "2348029999999"},
			want:     "2348029999999",
		},
		{
			name:     "phone-first: wallet phone equals reachable phone",
			snapshot: &registrationJobSnapshot{Phone: "2348010000000", WalletPhone: "2348010000000"},
			want:     "2348010000000",
		},
		{
			name:     "in-flight job without wallet phone falls back to reachable phone",
			snapshot: &registrationJobSnapshot{Phone: "2348010000000"},
			want:     "2348010000000",
		},
		{
			name:     "whitespace-only wallet phone falls back to reachable phone",
			snapshot: &registrationJobSnapshot{Phone: "2348010000000", WalletPhone: "   "},
			want:     "2348010000000",
		},
		{
			name:     "nil snapshot returns empty",
			snapshot: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := walletPhoneFromSnapshot(tt.snapshot); got != tt.want {
				t.Fatalf("walletPhoneFromSnapshot() = %q, want %q", got, tt.want)
			}
		})
	}
}

// When the wallet provider echoes back an empty phone, the stored wallet record
// must fall back to the BVN phone we actually sent — not the reachable phone.
func TestNormalizeWalletResponseFallsBackToWalletPhone(t *testing.T) {
	address := "1 Test Street"
	resp := &WalletResponse{
		Customer: &WalletCustomer{
			ID:          "cust-1",
			PhoneNumber: "", // provider returned no phone
			Address:     &address,
			Email:       "user@example.com",
			BVN:         "12345678901",
			FirstName:   "Ada",
			LastName:    "Obi",
			DateOfBirth: "1990-01-01",
		},
		Wallet: &WalletInfo{
			WalletId:      "wallet-1",
			AccountNumber: "0123456789",
		},
	}
	snapshot := &registrationJobSnapshot{
		Phone:       "2348010000000", // reachable/alternate phone
		WalletPhone: "2348029999999", // BVN-linked phone
	}

	if err := normalizeWalletResponse(resp, snapshot); err != nil {
		t.Fatalf("normalizeWalletResponse returned error: %v", err)
	}

	if resp.Customer.PhoneNumber != snapshot.WalletPhone {
		t.Fatalf("wallet-record phone = %q, want BVN phone %q", resp.Customer.PhoneNumber, snapshot.WalletPhone)
	}
}
