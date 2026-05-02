package user

import (
	"errors"
	"strings"
	"time"
)

func NormalizeDOB(input string) (string, error) {
	input = strings.TrimSpace(input)

	layouts := []string{
		"02-01-2006",
		"02/01/2006",
		"2006-01-02",
		"02012006",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, input)
		if err != nil {
			return t.Format("2006-01-02"), nil
		}
	}

	return "", errors.New("invalid dob")
}
