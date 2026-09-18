package optimus

import "testing"

func TestIsIntrabank(t *testing.T) {
	tests := []struct {
		name                string
		beneficiaryBankCode string
		sourceBankCode      string
		want                bool
	}{
		{"same bank code is intrabank", "000036", "000036", true},
		{"different bank code is interbank", "100040", "000036", false},
		{"case-insensitive match still intrabank", "AbC", "abc", true},
		{"empty beneficiary code is never intrabank", "", "000036", false},
		{"whitespace-padded codes still match", " 000036 ", "000036", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIntrabank(tt.beneficiaryBankCode, tt.sourceBankCode)
			if got != tt.want {
				t.Fatalf("isIntrabank(%q, %q) = %v, want %v", tt.beneficiaryBankCode, tt.sourceBankCode, got, tt.want)
			}
		})
	}
}
