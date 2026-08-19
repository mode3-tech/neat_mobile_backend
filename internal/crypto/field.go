// Package crypto provides field-level encryption for sensitive DB columns
// (BVN/NIN) using AES-256-GCM, plus a deterministic hash for lookup/dedup
// columns that can no longer query the encrypted value directly.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

// versionPrefix lets Decrypt tell ciphertext apart from legacy plaintext rows
// written before encryption was introduced, so a mixed-state table (some rows
// encrypted, some not yet backfilled) still reads back correctly.
const versionPrefix = "v1:"

type FieldCipher struct {
	gcm cipher.AEAD
}

// NewFieldCipher builds a cipher from a raw 32-byte AES-256 key.
func NewFieldCipher(key []byte) (*FieldCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &FieldCipher{gcm: gcm}, nil
}

// NewFieldCipherFromBase64 builds a cipher from a base64-encoded 32-byte key,
// the form BVN_NIN_ENCRYPTION_KEY is stored in (e.g. output of `openssl rand
// -base64 32`).
func NewFieldCipherFromBase64(encoded string) (*FieldCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("crypto: encryption key is not valid base64")
	}
	return NewFieldCipher(key)
}

// Encrypt returns a versioned, base64-encoded ciphertext safe to store in a
// text column. Encrypt("") returns "" so empty/unset fields round-trip
// cleanly without needing special-casing at call sites.
func (c *FieldCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return versionPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. A value without the version prefix is treated as
// legacy plaintext (a row written before encryption, or not yet backfilled)
// and returned unchanged rather than failing.
func (c *FieldCipher) Decrypt(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, versionPrefix) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, versionPrefix))
	if err != nil {
		return "", err
	}
	nonceSize := c.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Hash returns a deterministic SHA-256 hex digest for lookup/uniqueness
// columns. It never protects the value at rest on its own - Encrypt does
// that - it only lets an encrypted column still be queried by equality.
func Hash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
