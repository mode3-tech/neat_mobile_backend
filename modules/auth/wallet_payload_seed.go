package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

func BuildWalletPayloadSeed(secretKey string, walletInfo *WalletPayload) (string, error) {
	if walletInfo == nil {
		return "", errors.New("wallet payload is required")
	}

	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return "", nil
	}

	email := strings.ToLower(strings.TrimSpace(walletInfo.Email))
	phone := strings.TrimSpace(walletInfo.PhoneNumber)
	dob := strings.TrimSpace(walletInfo.DateOfBirth)
	bvn := strings.TrimSpace(walletInfo.BVN)

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(bvn))
	h.Write([]byte("|"))
	h.Write([]byte(email))
	h.Write([]byte("|"))
	h.Write([]byte(phone))
	h.Write([]byte("|"))
	h.Write([]byte(dob))

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:8], nil
}

func SeedWalletPayload(walletInfo *WalletPayload, secretKey string, usePrefix bool) (*WalletPayload, error) {
	if walletInfo == nil {
		return nil, errors.New("wallet payload is required")
	}

	seed, err := BuildWalletPayloadSeed(secretKey, walletInfo)
	if err != nil {
		return nil, err
	}

	copyPayload := *walletInfo
	if walletInfo.Metadata != nil {
		copyPayload.Metadata = make(map[string]interface{}, len(walletInfo.Metadata)+1)
		for k, v := range walletInfo.Metadata {
			copyPayload.Metadata[k] = v
		}
	} else {
		copyPayload.Metadata = make(map[string]interface{})
	}

	if seed == "" {
		return &copyPayload, nil
	}

	copyPayload.Email = decorateEmailWithSeed(walletInfo.Email, seed, usePrefix)
	copyPayload.Address = decorateStringWithSeed(walletInfo.Address, seed, usePrefix)
	copyPayload.Metadata["wallet_generation_seed"] = seed

	return &copyPayload, nil
}

func decorateEmailWithSeed(email, seed string, usePrefix bool) string {
	email = strings.TrimSpace(email)
	if email == "" || seed == "" {
		return email
	}

	parts := strings.SplitN(email, "@", 2)
	local := parts[0]
	domain := ""
	if len(parts) == 2 {
		domain = parts[1]
	}

	if usePrefix {
		local = seed + "." + local
	} else {
		local = local + "+" + seed
	}

	if domain == "" {
		return local
	}
	return local + "@" + domain
}

func decorateStringWithSeed(value, seed string, usePrefix bool) string {
	value = strings.TrimSpace(value)
	if value == "" || seed == "" {
		return value
	}

	if usePrefix {
		return seed + "-" + value
	}
	return value + "-" + seed
}
