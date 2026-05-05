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
	return hex.EncodeToString(sum)[:10], nil
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

	copyPayload.FirstName = decorateStringWithSeed(walletInfo.FirstName, seed, usePrefix)
	copyPayload.LastName = decorateStringWithSeed(walletInfo.LastName, seed, usePrefix)
	copyPayload.PhoneNumber = decoratePhoneNumberWithSeed(walletInfo.PhoneNumber, seed, usePrefix)
	copyPayload.BVN = decorateBVNWithSeed(seed)
	copyPayload.Email = decorateEmailWithSeed(walletInfo.Email, seed, usePrefix)
	copyPayload.Address = decorateStringWithSeed(walletInfo.Address, seed, usePrefix)
	copyPayload.Metadata["wallet_generation_seed"] = seed

	return &copyPayload, nil
}

func decorateBVNWithSeed(seed string) string {
	return normalizeSeedDigits(seed, 11)
}

func decoratePhoneNumberWithSeed(phone, seed string, usePrefix bool) string {
	if seed == "" {
		return normalizePhoneTo234(phone)
	}

	seedDigits := normalizeSeedDigits(seed, 10)
	return "234" + seedDigits
}

func normalizePhoneTo234(phone string) string {
	digits := normalizeDigits(phone)
	if strings.HasPrefix(digits, "234") {
		return digits
	}
	if len(digits) > 10 {
		digits = digits[len(digits)-10:]
	}
	return "234" + digits
}

func normalizeDigits(value string) string {
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

func truncatePhoneNumber(phone string, maxLen int, usePrefix bool) string {
	if len(phone) <= maxLen {
		return phone
	}
	if usePrefix {
		return phone[:maxLen]
	}
	return phone[len(phone)-maxLen:]
}

func normalizeSeedDigits(seed string, length int) string {
	var digits strings.Builder
	for _, ch := range seed {
		if digits.Len() >= length {
			break
		}
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
			continue
		}
		if ch >= 'a' && ch <= 'f' {
			digits.WriteRune('0' + ((ch - 'a') % 10))
			continue
		}
		if ch >= 'A' && ch <= 'F' {
			digits.WriteRune('0' + ((ch - 'A') % 10))
			continue
		}
	}
	for digits.Len() < length {
		digits.WriteByte('0')
	}
	return digits.String()[:length]
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
