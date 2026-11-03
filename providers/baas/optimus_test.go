package baas

import "testing"

func TestOptimusWalletAPIURL(t *testing.T) {
	const want = "https://h/opti-finserve-api/v1/Customer/validate-bvn"
	bases := []string{
		"https://h/opti-finserve-api/v1",
		"https://h/opti-finserve-api/v1/",
		"https://h/opti-finserve-api",
	}
	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			o := &Optimus{WalletBaseURL: base}
			if got := o.walletAPIURL("/Customer/validate-bvn"); got != want {
				t.Fatalf("walletAPIURL(%q) = %q, want %q", base, got, want)
			}
		})
	}
}
