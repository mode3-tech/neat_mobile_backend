package wema

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"neat_mobile_app_backend/internal/modules/wallet"
)

// Signer purposes. Each signs a different kind of value, so a token minted
// for one use can never be replayed as another.
const (
	// PurposeSecurityInfo signs the transaction reference sent to ALAT as
	// securityInfo. ALAT echoes it back to our Authentication Callback, which
	// approves the debit/payout only if the signature verifies.
	PurposeSecurityInfo = wallet.SignPurposeSecurityInfo
	// PurposeStatement binds an ALAT statement reference ID to the user who
	// requested it, so one user can't fetch another user's statement.
	PurposeStatement = wallet.SignPurposeStatement
)

// Signer issues and checks HMAC-SHA256 tokens keyed by ALAT_SECURITY_SECRET.
// ALAT leaves the securityInfo format to the client ("an encrypted value whose
// signature is known only by the client"), so an HMAC we alone can verify is
// enough.
type Signer struct {
	secret []byte
}

// NewSigner returns a Signer. An empty secret yields a Signer whose Sign
// always fails, so Wema money movement fails loudly rather than unsigned.
func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(strings.TrimSpace(secret))}
}

// Configured reports whether a secret is set.
func (s *Signer) Configured() bool {
	return s != nil && len(s.secret) > 0
}

// Sign returns a URL-safe signature of value for purpose.
func (s *Signer) Sign(purpose, value string) (string, error) {
	if !s.Configured() {
		return "", errors.New("wema: ALAT_SECURITY_SECRET is not configured")
	}
	if strings.TrimSpace(value) == "" {
		return "", errors.New("wema: cannot sign an empty value")
	}
	return base64.RawURLEncoding.EncodeToString(s.mac(purpose, value)), nil
}

// Verify reports whether signature is Sign(purpose, value), in constant time.
func (s *Signer) Verify(purpose, value, signature string) bool {
	if !s.Configured() || strings.TrimSpace(value) == "" {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	return hmac.Equal(got, s.mac(purpose, value))
}

func (s *Signer) mac(purpose, value string) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(purpose))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return m.Sum(nil)
}
