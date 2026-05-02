package phone

import (
	"errors"
	"strings"
)

func MaskPhone(phone string) (string, error) {
	normalizedPhone, err := NormalizeNigerianNumber(phone)
	if err != nil {
		return "", err
	}

	if len(normalizedPhone) <= 4 {
		return "", errors.New("normalized phone is too short")
	}

	last4 := normalizedPhone[len(normalizedPhone)-4:]
	masked := strings.Repeat("*", len(normalizedPhone)-4)

	return masked + last4, nil
}
