package crypto

import "testing"

func testCipher(t *testing.T) *FieldCipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewFieldCipher(key)
	if err != nil {
		t.Fatalf("NewFieldCipher: %v", err)
	}
	return c
}

func TestFieldCipher_RoundTrip(t *testing.T) {
	c := testCipher(t)

	ciphertext, err := c.Encrypt("12345678901")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ciphertext == "12345678901" {
		t.Fatal("Encrypt returned plaintext unchanged")
	}

	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "12345678901" {
		t.Fatalf("expected round-tripped plaintext, got %q", plaintext)
	}
}

func TestFieldCipher_EmptyStringRoundTrips(t *testing.T) {
	c := testCipher(t)

	ciphertext, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ciphertext != "" {
		t.Fatalf("expected empty ciphertext for empty plaintext, got %q", ciphertext)
	}

	plaintext, err := c.Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "" {
		t.Fatalf("expected empty plaintext, got %q", plaintext)
	}
}

func TestFieldCipher_DecryptLegacyPlaintextPassesThrough(t *testing.T) {
	c := testCipher(t)

	// Rows written before encryption was introduced have no version prefix -
	// Decrypt must return them unchanged instead of erroring, so mixed-state
	// tables (some rows encrypted, some not yet backfilled) keep working.
	plaintext, err := c.Decrypt("12345678901")
	if err != nil {
		t.Fatalf("Decrypt legacy plaintext: %v", err)
	}
	if plaintext != "12345678901" {
		t.Fatalf("expected legacy plaintext passthrough, got %q", plaintext)
	}
}

func TestFieldCipher_EncryptIsNonDeterministic(t *testing.T) {
	c := testCipher(t)

	a, err := c.Encrypt("12345678901")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	b, err := c.Encrypt("12345678901")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if a == b {
		t.Fatal("expected different ciphertexts for the same plaintext (random nonce per call)")
	}
}

func TestHash_DeterministicAndDistinct(t *testing.T) {
	a := Hash("12345678901")
	b := Hash("12345678901")
	if a != b {
		t.Fatalf("expected Hash to be deterministic, got %q and %q", a, b)
	}
	if Hash("12345678902") == a {
		t.Fatal("expected different inputs to hash differently")
	}
	if Hash(" 12345678901 ") != a {
		t.Fatal("expected Hash to trim whitespace like the rest of the codebase's hashing does")
	}
}
