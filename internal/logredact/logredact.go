// Package logredact renders sensitive request data (BVN, NIN, phone, email,
// selfies) in a form that is safe to write to logs and Sentry.
package logredact

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Tail keeps only the last 4 characters (BVN, NIN).
func Tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// PhoneShape keeps a phone's first 3 and last 4 digits, so the format
// (080..., 234...) is visible without logging the number.
func PhoneShape(p string) string {
	p = strings.TrimSpace(p)
	if len(p) <= 7 {
		return "***"
	}
	return p[:3] + strings.Repeat("*", len(p)-7) + p[len(p)-4:]
}

// EmailShape keeps an email's first character and its domain.
func EmailShape(e string) string {
	e = strings.TrimSpace(e)
	at := strings.LastIndex(e, "@")
	if at < 1 {
		return "***"
	}
	return e[:1] + strings.Repeat("*", at-1) + e[at:]
}

// Image summarises a base64 selfie without logging it: its length, whether it
// carries a data: URI prefix (ALAT may want raw base64), and its first
// characters, which are the file header and identify nothing.
func Image(img string) string {
	img = strings.TrimSpace(img)
	if img == "" {
		return "<empty>"
	}
	head := img
	if len(head) > 12 {
		head = head[:12]
	}
	return fmt.Sprintf("<%d chars, data_uri=%t, starts=%q>", len(img), strings.HasPrefix(img, "data:"), head)
}

// Tier2Payload renders the ALAT tier 2 (face) account creation payload as
// JSON, as sent, so it can be shared with Wema when debugging. BVN, NIN, phone
// and email are NOT redacted. Only the selfie is summarised (size and
// prefix): it is hundreds of KB of base64. phone is logged as given, so pass
// it in the format ALAT will receive.
func Tier2Payload(bvn, nin, phone, email, image string, hasAddress bool, correlationID string) string {
	addr := "omitted"
	if hasAddress {
		addr = "provided"
	}
	b, err := json.Marshal(map[string]any{
		"bvn":                bvn,
		"nin":                nin,
		"phoneNumber":        phone,
		"emailAddress":       email,
		"residentialAddress": addr,
		"liveImageOfFace":    Image(image),
		"correlationId":      correlationID,
	})
	if err != nil {
		return "<unprintable>"
	}
	return string(b)
}
