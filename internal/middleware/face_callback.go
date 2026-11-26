package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	faceCallbackRateLimit  = 30
	faceCallbackRateWindow = time.Minute
)

// FaceCallbackGuard protects Wema's facial-verification callback. Wema's face
// web app reports its result from the customer's browser, so the caller's IP
// can't be allowlisted. A call is admitted when its Origin header is one of
// allowedOrigins (comma-separated) or its source IP is in allowedIPs (in case
// Wema also calls server-to-server), and each source IP is limited to 30
// calls a minute. Origin is only a guard against other websites - a script can
// forge it - so the handler still trusts nothing beyond storing the payload;
// ALAT rejects a correlationId it didn't issue when the account is created.
//
// trustedProxies is read as in AlatWebhookIPAllowlist. With neither
// allowedOrigins nor allowedIPs set, every call is rejected.
func FaceCallbackGuard(allowedOrigins, allowedIPs, trustedProxies string) gin.HandlerFunc {
	origins := map[string]bool{}
	for _, o := range strings.Split(allowedOrigins, ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			origins[o] = true
		}
	}
	allowedSet, err := parsePrefixes(allowedIPs)
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR invalid ALAT_WEBHOOK_ALLOWED_IPS: %v", err)
		allowedSet = nil
	}
	proxySet, err := parsePrefixes(trustedProxies)
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR invalid ALAT_WEBHOOK_TRUSTED_PROXIES: %v; ignoring X-Forwarded-For", err)
		proxySet = nil
	}
	if len(origins) == 0 {
		log.Print("[ALAT webhook] level=WARN ALAT_FACE_ALLOWED_ORIGINS is not set; only allowlisted IPs can post facial-verification results")
	}

	var mu sync.Mutex
	hits := map[string][]time.Time{}

	return func(c *gin.Context) {
		ip, ok := sourceIP(c.Request, proxySet)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if !origins[origin] && !containsAddr(allowedSet, ip) {
			log.Printf("[ALAT webhook] level=WARN rejected %s %s from ip=%s origin=%q user_agent=%q",
				c.Request.Method, c.Request.URL.Path, ip, origin, c.GetHeader("User-Agent"))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		now := time.Now()
		key := ip.String()
		mu.Lock()
		recent := hits[key][:0]
		for _, t := range hits[key] {
			if now.Sub(t) < faceCallbackRateWindow {
				recent = append(recent, t)
			}
		}
		limited := len(recent) >= faceCallbackRateLimit
		if !limited {
			recent = append(recent, now)
		}
		if len(recent) == 0 {
			delete(hits, key)
		} else {
			hits[key] = recent
		}
		mu.Unlock()
		if limited {
			log.Printf("[ALAT webhook] level=WARN rate limited facial-verification ip=%s", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}
