package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestVerifyDeviceSignature_Valid(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	challenge := "challenge-token"
	signature := ed25519.Sign(privateKey, []byte(challenge))

	valid, err := verifyDeviceSignature(
		base64.StdEncoding.EncodeToString(publicKey),
		challenge,
		base64.StdEncoding.EncodeToString(signature),
	)
	if err != nil {
		t.Fatalf("verifyDeviceSignature returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected signature to be valid")
	}
}

func TestVerifyDeviceSignature_InvalidSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	challenge := "challenge-token"
	signature := ed25519.Sign(privateKey, []byte("different-challenge"))

	valid, err := verifyDeviceSignature(
		base64.StdEncoding.EncodeToString(publicKey),
		challenge,
		base64.StdEncoding.EncodeToString(signature),
	)
	if err != nil {
		t.Fatalf("verifyDeviceSignature returned error: %v", err)
	}
	if valid {
		t.Fatal("expected signature to be invalid")
	}
}

func TestVerifyDeviceSignature_RejectsBadEncoding(t *testing.T) {
	_, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	_, verifyErr := verifyDeviceSignature("not-a-key", "challenge-token", "not-a-signature")
	if verifyErr == nil {
		t.Fatal("expected error for invalid encoded inputs")
	}
}
