package registerv2

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
)

// defaultAlatBackoff is used when ALAT rate-limits us without saying for how
// long.
const defaultAlatBackoff = 90 * time.Second

var alatRetryAfterRe = regexp.MustCompile(`(?i)try again in (\d+) second`)

// alatRateLimitDelay reports whether err is ALAT's 429 ("Rate limit is
// exceeded. Try again in N seconds.") and how long it asked us to wait.
func alatRateLimitDelay(err error) (time.Duration, bool) {
	var alatErr *appErr.AlatError
	if !errors.As(err, &alatErr) {
		return 0, false
	}
	if alatErr.StatusCode != "429" && !strings.Contains(strings.ToLower(alatErr.Message), "rate limit") {
		return 0, false
	}
	if m := alatRetryAfterRe.FindStringSubmatch(alatErr.Message); m != nil {
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n > 0 {
			return time.Duration(n) * time.Second, true
		}
	}
	return defaultAlatBackoff, true
}

// noteAlatRateLimit starts a pause on ALAT lookups if err is a 429, and
// reports whether it was.
func (s *Service) noteAlatRateLimit(err error) bool {
	d, ok := alatRateLimitDelay(err)
	if !ok {
		return false
	}
	until := time.Now().Add(d + 2*time.Second) // a little slack past ALAT's window
	s.alatBackoffMu.Lock()
	if until.After(s.alatBackoffUntil) {
		s.alatBackoffUntil = until
	}
	s.alatBackoffMu.Unlock()
	return true
}

// alatBackoffRemaining is how long ALAT lookups are paused for; 0 if not.
func (s *Service) alatBackoffRemaining() time.Duration {
	s.alatBackoffMu.Lock()
	defer s.alatBackoffMu.Unlock()
	if d := time.Until(s.alatBackoffUntil); d > 0 {
		return d
	}
	return 0
}
