package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestVerifyDeviceSignature_Valid(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	publicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey returned error: %v", err)
	}

	challenge := "challenge-token"
	digest := sha256.Sum256([]byte(challenge))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("SignASN1 returned error: %v", err)
	}

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
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	publicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey returned error: %v", err)
	}

	challenge := "challenge-token"
	digest := sha256.Sum256([]byte("different-challenge"))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("SignASN1 returned error: %v", err)
	}

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
	_, verifyErr := verifyDeviceSignature("not-a-key", "challenge-token", "not-a-signature")
	if verifyErr == nil {
		t.Fatal("expected error for invalid encoded inputs")
	}
}
