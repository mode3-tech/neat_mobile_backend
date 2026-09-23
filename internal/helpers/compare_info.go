package helpers

import (
	"regexp"
	"strings"
	"time"
)

var dobRegex = regexp.MustCompile(`[/-]`)

// dobLayouts are the full-date layouts tried when comparing identity-provider
// DOBs (e.g. BVN, NIN). Different providers may format DOB differently
// (numeric, ISO, or named-month like "16-Aug-2002"), so all known shapes are
// tried here; they're unambiguous relative to each other (numeric day/month
// vs. leading 4-digit year vs. a month name in one position or the other).
var dobLayouts = []string{
	"02-01-2006",      // DD-MM-YYYY
	"2006-01-02",      // YYYY-MM-DD
	"02-Jan-2006",     // DD-Mon-YYYY, e.g. 16-Aug-2002
	"Jan-02-2006",     // Mon-DD-YYYY, e.g. Aug-16-2002
	"02-January-2006", // DD-Month-YYYY, e.g. 16-August-2002
	"January-02-2006", // Month-DD-YYYY, e.g. August-16-2002
}

// parseDOB tries each known identity-provider DOB layout in turn.
func parseDOB(value string) (time.Time, bool) {
	clean := strings.ReplaceAll(strings.TrimSpace(value), "/", "-")
	for _, layout := range dobLayouts {
		if dob, err := time.Parse(layout, clean); err == nil {
			return dob, true
		}
	}
	return time.Time{}, false
}

// serializeDOB strips separators so two DOB strings that failed layout
// parsing can still be compared as a last resort.
func serializeDOB(dob string) string {
	return dobRegex.ReplaceAllString(strings.TrimSpace(dob), "")
}

// NamesMatch compares two names case-insensitively, collapsing repeated
// whitespace first (e.g. a double space from an empty middle name on one
// side but not the other shouldn't count as a mismatch).
func NamesMatch(a, b string) bool {
	normalize := func(s string) string {
		return strings.ToLower(strings.Join(strings.Fields(s), " "))
	}
	return normalize(a) == normalize(b)
}

// DOBsMatch compares two DOB strings that may come from providers using
// different date formats/orders (e.g. one as DD-MM-YYYY, another as
// YYYY-MM-DD). It parses both into actual calendar dates and compares those
// instead of comparing raw strings, so a format difference alone doesn't
// look like a mismatch. Falls back to a strip-separators comparison if
// either side doesn't match a known DOB layout.
func DOBsMatch(a, b string) bool {
	aDate, aOK := parseDOB(a)
	bDate, bOK := parseDOB(b)
	if aOK && bOK {
		return aDate.Year() == bDate.Year() && aDate.Month() == bDate.Month() && aDate.Day() == bDate.Day()
	}
	return serializeDOB(a) == serializeDOB(b)
}
