package phone

import (
	appErr "neat_mobile_app_backend/internal/errors"
	"regexp"
	"strings"
)

var nonDigit = regexp.MustCompile(`\D`)

func NormalizeNigerianNumber(input string) (string, error) {
	cleaned := nonDigit.ReplaceAllString(strings.TrimSpace(input), "")
	if cleaned == "" {
		return "", appErr.ErrInvalidPhoneNumber
	}

	switch {
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		return "234" + cleaned[1:], nil
	case strings.HasPrefix(cleaned, "234") && len(cleaned) == 13:
		return cleaned, nil
	case len(cleaned) == 10:
		return "234" + cleaned, nil
	}

	return "", appErr.ErrInvalidPhoneNumber

}

func ToLocalFormat(input string) (string, error) {
	cleaned := nonDigit.ReplaceAllString(strings.TrimSpace(input), "")
	switch {
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		return cleaned, nil
	case strings.HasPrefix(cleaned, "234") && len(cleaned) == 13:
		return "0" + cleaned[3:], nil
	case len(cleaned) == 10:
		return "0" + cleaned, nil
	}
	return "", appErr.ErrInvalidPhoneNumber
}
