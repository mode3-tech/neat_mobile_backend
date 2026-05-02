package phone

import (
	"errors"
	"regexp"
	"strings"
)

var nonDigit = regexp.MustCompile(`\D`)

func NormalizeNigerianNumber(input string) (string, error) {
	cleaned := nonDigit.ReplaceAllString(strings.TrimSpace(input), "")
	if cleaned == "" {
		return "", errors.New("invalid Nigerian number")
	}

	switch {
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		return "234" + cleaned[1:], nil
	case strings.HasPrefix(cleaned, "234") && len(cleaned) == 13:
		return cleaned, nil
	case len(cleaned) == 10:
		return "234" + cleaned, nil
	}

	return "", errors.New("invalid Nigerian number")

}
