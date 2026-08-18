package timeutil

import (
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
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

// dobFullDateLayouts are the day-month-year layouts ParseDOB accepts, tried in
// order. Includes numeric (DD-MM-YYYY) and named-month variants (DD-Mon-YYYY,
// Mon-DD-YYYY, and full month name) since provider responses (e.g. BVN/NIN)
// aren't always numeric-only, e.g. "16-Aug-2002". Deliberately does NOT
// include YYYY-MM-DD - see the "invalid format" test case in age_test.go.
var dobFullDateLayouts = []string{
	"02-01-2006",      // DD-MM-YYYY
	"02-Jan-2006",     // DD-Mon-YYYY, e.g. 16-Aug-2002
	"Jan-02-2006",     // Mon-DD-YYYY, e.g. Aug-16-2002
	"02-January-2006", // DD-Month-YYYY, e.g. 16-August-2002
	"January-02-2006", // Month-DD-YYYY, e.g. August-16-2002
}

// ParseDOB parses DOB in DD-MM-YYYY, DD/MM/YYYY, DD-Mon-YYYY (and slash/full-month
// variants), YYYY-MM, YYYY/MM, MM-YYYY, or MM/YYYY format. For month-year or
// year-month inputs, day is assumed to be 01.
func ParseDOB(value string) (time.Time, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return time.Time{}, appErr.ErrInvalidDOB
	}

	clean = strings.ReplaceAll(clean, "/", "-")

	for _, layout := range dobFullDateLayouts {
		if dob, err := time.Parse(layout, clean); err == nil {
			return dob, nil
		}
	}

	yearMonthDOB, err := time.Parse("2006-01", clean)
	if err == nil {
		return yearMonthDOB, nil
	}

	monthYearDOB, err := time.Parse("01-2006", clean)
	if err == nil {
		return monthYearDOB, nil
	}

	log.Printf("invalid dob format %q: expected DD-MM-YYYY, DD/MM/YYYY, DD-Mon-YYYY, YYYY-MM, YYYY/MM, MM-YYYY or MM/YYYY", value)
	return time.Time{}, appErr.ErrInvalidDOB
}

func AgeFromDOBString(value string, now time.Time) (int, error) {
	dob, err := ParseDOB(value)
	if err != nil {
		log.Printf("error parsing dob %q: %v", value, err)
		return 0, appErr.ErrInvalidDOB
	}

	return AgeFromDOB(dob, now), nil
}
