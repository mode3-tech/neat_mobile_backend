package baas

import (
	"errors"
	"testing"
)

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

func TestDecodeOptimusEnvelopePlaintextError(t *testing.T) {
	o := &Optimus{}
	cases := map[string]string{
		"quoted body": `"Decryption failed or empty data."`,
		"raw body":    `Decryption failed or empty data.`,
		// valid base64 but not a PGP message
		"base64-ish word": `"Unauthorized"`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := o.decodeOptimusEnvelope([]byte(body))
			var plainErr *OptimusPlaintextError
			if !errors.As(err, &plainErr) {
				t.Fatalf("expected *OptimusPlaintextError, got %T: %v", err, err)
			}
			if plainErr.Body == "" {
				t.Fatal("expected non-empty plaintext body")
			}
		})
	}
}

func TestDecodeOptimusEnvelopePlainJSONPassthrough(t *testing.T) {
	o := &Optimus{}
	body := []byte(`{"responseCode":"00"}`)
	got, err := o.decodeOptimusEnvelope(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %s, want %s", got, body)
	}
}
