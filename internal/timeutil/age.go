package timeutil

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func AgeFromDOB(dob time.Time, now time.Time) int {
	if dob.After(now) {
		return 0
	}

	age := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		age--
	}
	return age
}

// ParseDOB parses DOB in DD-MM-YYYY, DD/MM/YYYY, YYYY-MM, or YYYY/MM format.
// For year-month inputs, day is assumed to be 01.
func ParseDOB(value string) (time.Time, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return time.Time{}, fmt.Errorf("dob is required")
	}

	clean = strings.ReplaceAll(clean, "/", "-")
	dob, err := time.Parse("02-01-2006", clean)
	if err == nil {
		return dob, nil
	}

	yearMonthDOB, err := time.Parse("2006-01", clean)
	if err == nil {
		return yearMonthDOB, nil
	}

	return time.Time{}, fmt.Errorf("invalid dob format %q: expected DD-MM-YYYY, DD/MM/YYYY, YYYY-MM or YYYY/MM", value)
}

func AgeFromDOBString(value string, now time.Time) (int, error) {
	dob, err := ParseDOB(value)
	if err != nil {
		return 0, errors.New("unable to get age from dob, check dob again")
	}

	return AgeFromDOB(dob, now), nil
}
