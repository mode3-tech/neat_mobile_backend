package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"hash/fnv"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultLoginRateLimitIPMaxAttempts    = 20
	defaultLoginRateLimitEmailMaxAttempts = 5
	defaultLoginRateLimitWindow           = 15 * time.Minute
	defaultLoginRateLimitBlockDuration    = 15 * time.Minute
	defaultLoginRateLimitShards           = 64
	defaultLoginRateLimitIPMaxKeys        = 100_000
	defaultLoginRateLimitEmailMaxKeys     = 100_000
	defaultLoginRateLimitCleanupInterval  = 1 * time.Minute
	defaultLoginRateLimitErrorMessage     = "too many login attempts, please try again later"
)

type LoginRateLimiterConfig struct {
	IPMaxAttempts    int
	EmailMaxAttempts int
	Window           time.Duration
	BlockDuration    time.Duration
	Shards           int
	IPMaxKeys        int
	EmailMaxKeys     int
	CleanupInterval  time.Duration
}

type loginAttemptState struct {
	Count        int
	WindowStart  time.Time
	BlockedUntil time.Time
	LastSeen     time.Time
}

type attemptShard struct {
	mu          sync.Mutex
	entries     map[string]loginAttemptState
	lastCleanup time.Time
}

type attemptStore struct {
	shards          []attemptShard
	maxKeys         int
	window          time.Duration
	cleanupInterval time.Duration
}

type LoginRateLimiter struct {
	cfg   LoginRateLimiterConfig
	nowFn func() time.Time

	ipAttempts    attemptStore
	emailAttempts attemptStore
}

func NewLoginRateLimiter(cfg LoginRateLimiterConfig) *LoginRateLimiter {
	cfg = withDefaultConfig(cfg)

	return &LoginRateLimiter{
		cfg:           cfg,
		nowFn:         time.Now,
		ipAttempts:    newAttemptStore(cfg.Shards, cfg.IPMaxKeys, cfg.Window, cfg.CleanupInterval),
		emailAttempts: newAttemptStore(cfg.Shards, cfg.EmailMaxKeys, cfg.Window, cfg.CleanupInterval),
	}
}

func withDefaultConfig(cfg LoginRateLimiterConfig) LoginRateLimiterConfig {
	if cfg.IPMaxAttempts <= 0 {
		cfg.IPMaxAttempts = defaultLoginRateLimitIPMaxAttempts
	}
	if cfg.EmailMaxAttempts <= 0 {
		cfg.EmailMaxAttempts = defaultLoginRateLimitEmailMaxAttempts
	}
	if cfg.Window <= 0 {
		cfg.Window = defaultLoginRateLimitWindow
	}
	if cfg.BlockDuration <= 0 {
		cfg.BlockDuration = defaultLoginRateLimitBlockDuration
	}
	if cfg.Shards <= 0 {
		cfg.Shards = defaultLoginRateLimitShards
	}
	if cfg.IPMaxKeys <= 0 {
		cfg.IPMaxKeys = defaultLoginRateLimitIPMaxKeys
	}
	if cfg.EmailMaxKeys <= 0 {
		cfg.EmailMaxKeys = defaultLoginRateLimitEmailMaxKeys
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = defaultLoginRateLimitCleanupInterval
	}

	return cfg
}

func newAttemptStore(shards, maxKeys int, window, cleanupInterval time.Duration) attemptStore {
	store := attemptStore{
		shards:          make([]attemptShard, shards),
		maxKeys:         maxKeys,
		window:          window,
		cleanupInterval: cleanupInterval,
	}

	for i := range store.shards {
		store.shards[i].entries = make(map[string]loginAttemptState)
	}

	return store
}

func (l *LoginRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := normalizeIP(c.ClientIP())
		email := l.extractEmail(c)
		now := l.nowFn().UTC()

		if blockedUntil, blocked := l.nextBlockedUntil(ip, email, now); blocked {
			retryAfter := max(int(blockedUntil.Sub(now).Seconds()), 1)

			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":               defaultLoginRateLimitErrorMessage,
				"retry_after_seconds": retryAfter,
			})
			return
		}

		c.Next()

		switch c.Writer.Status() {
		case http.StatusUnauthorized:
			l.recordFailure(ip, email, l.nowFn().UTC())
		default:
			if c.Writer.Status() >= http.StatusOK && c.Writer.Status() < http.StatusMultipleChoices {
				l.reset(ip, email)
			}
		}
	}
}

func normalizeIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return "unknown"
	}

	return trimmed
}

// func normalizeEmail(email string) string {
// 	return strings.ToLower(strings.TrimSpace(email))
// }

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

func (l *LoginRateLimiter) extractEmail(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return ""
	}

	var payload struct {
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return ""
	}

	phone, err := NormalizeNigerianNumber(payload.Phone)
	if err != nil {
		return ""
	}

	return (phone)
}

func (l *LoginRateLimiter) nextBlockedUntil(ip, email string, now time.Time) (time.Time, bool) {
	var blockedUntil time.Time
	if until, blocked := l.ipAttempts.blockedUntil(ip, now); blocked {
		blockedUntil = until
	}
	if email != "" {
		if until, blocked := l.emailAttempts.blockedUntil(email, now); blocked && until.After(blockedUntil) {
			blockedUntil = until
		}
	}

	return blockedUntil, !blockedUntil.IsZero()
}

func (l *LoginRateLimiter) recordFailure(ip, email string, now time.Time) {
	l.ipAttempts.recordFailure(ip, l.cfg.IPMaxAttempts, now, l.cfg.BlockDuration)
	if email != "" {
		l.emailAttempts.recordFailure(email, l.cfg.EmailMaxAttempts, now, l.cfg.BlockDuration)
	}
}

func (l *LoginRateLimiter) reset(ip, email string) {
	l.ipAttempts.reset(ip)
	if email != "" {
		l.emailAttempts.reset(email)
	}
}

func (s *attemptStore) blockedUntil(key string, now time.Time) (time.Time, bool) {
	if key == "" {
		return time.Time{}, false
	}

	shard := s.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	s.cleanupLocked(shard, now)

	state, ok := shard.entries[key]
	if !ok {
		return time.Time{}, false
	}

	if state.BlockedUntil.After(now) {
		return state.BlockedUntil, true
	}

	if shouldEvictState(state, now, s.window) {
		delete(shard.entries, key)
	}

	return time.Time{}, false
}

func (s *attemptStore) recordFailure(key string, maxAttempts int, now time.Time, blockDuration time.Duration) {
	if key == "" || maxAttempts <= 0 {
		return
	}

	shard := s.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	s.cleanupLocked(shard, now)

	state, ok := shard.entries[key]
	if !ok {
		s.makeRoomLocked(shard, now)
		state = loginAttemptState{
			WindowStart: now,
		}
	}

	if state.BlockedUntil.After(now) {
		state.LastSeen = now
		shard.entries[key] = state
		return
	}

	if state.WindowStart.IsZero() || now.Sub(state.WindowStart) > s.window {
		state.Count = 0
		state.WindowStart = now
		state.BlockedUntil = time.Time{}
	}

	state.Count++
	if state.Count >= maxAttempts {
		state.BlockedUntil = now.Add(blockDuration)
		state.Count = 0
		state.WindowStart = time.Time{}
	}

	state.LastSeen = now
	shard.entries[key] = state
}

func (s *attemptStore) reset(key string) {
	if key == "" {
		return
	}

	shard := s.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.entries, key)
}

func (s *attemptStore) shard(key string) *attemptShard {
	index := shardIndex(key, len(s.shards))
	return &s.shards[index]
}

func shardIndex(key string, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(shardCount))
}

func (s *attemptStore) makeRoomLocked(shard *attemptShard, now time.Time) {
	maxPerShard := s.maxKeysPerShard()
	if maxPerShard <= 0 {
		return
	}

	for len(shard.entries) >= maxPerShard {
		evicted := false

		for key, state := range shard.entries {
			if shouldEvictState(state, now, s.window) {
				delete(shard.entries, key)
				evicted = true
				break
			}
		}

		if evicted {
			continue
		}

		var oldestKey string
		var oldestAt time.Time
		for key, state := range shard.entries {
			candidate := state.LastSeen
			if candidate.IsZero() {
				candidate = state.WindowStart
			}
			if candidate.IsZero() {
				candidate = state.BlockedUntil
			}

			if oldestKey == "" || candidate.Before(oldestAt) {
				oldestKey = key
				oldestAt = candidate
			}
		}

		if oldestKey == "" {
			return
		}
		delete(shard.entries, oldestKey)
	}
}

func (s *attemptStore) maxKeysPerShard() int {
	if len(s.shards) == 0 {
		return 0
	}

	maxPerShard := s.maxKeys / len(s.shards)
	if maxPerShard <= 0 {
		return 1
	}

	return maxPerShard
}

func (s *attemptStore) cleanupLocked(shard *attemptShard, now time.Time) {
	if !shard.lastCleanup.IsZero() && now.Sub(shard.lastCleanup) < s.cleanupInterval {
		return
	}

	shard.lastCleanup = now
	for key, state := range shard.entries {
		if shouldEvictState(state, now, s.window) {
			delete(shard.entries, key)
		}
	}
}

func shouldEvictState(state loginAttemptState, now time.Time, window time.Duration) bool {
	if state.BlockedUntil.After(now) {
		return false
	}

	if !state.BlockedUntil.IsZero() && !state.BlockedUntil.After(now) {
		return true
	}

	if state.WindowStart.IsZero() {
		return true
	}

	return now.Sub(state.WindowStart) > window
}
