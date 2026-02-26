package otp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var nonDigit = regexp.MustCompile(`\D`)

func NormalizeNigerianNumber(input string) (string, error) {
	if input == "" {
		return "", errors.New("empty phone number")
	}

	cleaned := nonDigit.ReplaceAllString(input, "")

	switch {
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		return "+234" + cleaned[1:], nil
	case strings.HasPrefix(cleaned, "234") && len(cleaned) == 13:
		return "+" + cleaned, nil
	case len(cleaned) == 10:
		return "+234" + cleaned, nil
	case strings.HasPrefix(cleaned, "+234") && len(cleaned) == 14:
		return "+234" + cleaned[3:], nil
	}

	return "", errors.New("invalid Nigerian number")

}

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

func NormalizeDestination(destination string, channel Channel) (string, error) {
	switch channel {
	case ChannelEmail:
		dst := strings.TrimSpace(strings.ToLower(destination))
		if dst == "" || !emailRegex.MatchString(dst) {
			return "", errors.New("invalid email")
		}
		return dst, nil
	case ChannelSMS:
		return destination, nil
	default:
		return "", errors.New("unsupported channel")
	}
}

func Generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))

	if err != nil {
		return "", errors.New("error generating OTP")
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func HashOTP(pepper string, purpose Purpose, destination string, otp string) (string, error) {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	isEmail := emailRegex.Match([]byte(destination))
	var dst string
	var err error
	if isEmail {
		dst = strings.TrimSpace(strings.ToLower(destination))
	} else {
		dst, err = NormalizeNigerianNumber(destination)
		if err != nil {
			return "", err
		}
	}

	msg := string(purpose) + "|" + dst + "|" + otp
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HashEqualHex(aHex, bHex string) bool {
	a, err1 := hex.DecodeString(aHex)
	b, err2 := hex.DecodeString(bHex)
	if err1 != nil || err2 != nil {
		return false
	}

	return hmac.Equal(a, b)
}
