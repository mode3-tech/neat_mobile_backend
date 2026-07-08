package wallet

import "testing"

func TestFundedBeneficiaryAccountNumber(t *testing.T) {
	tests := []struct {
		name string
		data AccountFundedData
		want string
	}{
		{
			name: "real transfer-in credits the beneficiary, not the payer",
			data: AccountFundedData{
				AccountNumber:            "9059375974", // payer (originator)
				BeneficiaryAccountNumber: "8891450872", // our customer
			},
			want: "8891450872",
		},
		{
			name: "documented single-account sample falls back to accountNumber",
			data: AccountFundedData{
				AccountNumber: "8869253566",
			},
			want: "8869253566",
		},
		{
			name: "blank/whitespace beneficiary falls back to accountNumber",
			data: AccountFundedData{
				AccountNumber:            "8869253566",
				BeneficiaryAccountNumber: "   ",
			},
			want: "8869253566",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fundedBeneficiaryAccountNumber(&tt.data); got != tt.want {
				t.Fatalf("fundedBeneficiaryAccountNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}
