package middleware

import (
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// AlatWebhookIPAllowlist admits ALAT webhook calls only from the IPs Wema
// calls from. Wema signs none of its callbacks, so the source IP is the only
// proof a call is theirs.
//
// allowed and trustedProxies are comma-separated IPs or CIDRs. The caller's IP
// is the TCP peer, unless the peer is one of our trustedProxies (e.g. the load
// balancer in front of the app), in which case it is the right-most
// X-Forwarded-For entry that isn't a trusted proxy. gin's ClientIP isn't used:
// by default it trusts X-Forwarded-For from anyone, which would let a caller
// spoof Wema's IP.
//
// An empty or invalid allowlist rejects every call, so a missing env var fails
// closed instead of opening the webhooks to the internet.
func AlatWebhookIPAllowlist(allowed, trustedProxies string) gin.HandlerFunc {
	allowedSet, err := parsePrefixes(allowed)
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR invalid ALAT_WEBHOOK_ALLOWED_IPS: %v; rejecting all ALAT webhooks", err)
		allowedSet = nil
	}
	proxySet, err := parsePrefixes(trustedProxies)
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR invalid ALAT_WEBHOOK_TRUSTED_PROXIES: %v; ignoring X-Forwarded-For", err)
		proxySet = nil
	}
	if len(allowedSet) == 0 {
		log.Print("[ALAT webhook] level=WARN ALAT_WEBHOOK_ALLOWED_IPS is not set; ALAT webhooks will reject all requests")
	}

	return func(c *gin.Context) {
		ip, ok := sourceIP(c.Request, proxySet)
		if !ok || !containsAddr(allowedSet, ip) {
			log.Printf("[ALAT webhook] level=WARN rejected %s %s from ip=%s peer=%s",
				c.Request.Method, c.Request.URL.Path, ip, c.Request.RemoteAddr)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// sourceIP returns the caller's IP: the TCP peer, or, when the peer is a
// trusted proxy, the right-most X-Forwarded-For hop not in trustedProxies.
func sourceIP(r *http.Request, trustedProxies []netip.Prefix) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	peer = peer.Unmap()
	if !containsAddr(trustedProxies, peer) {
		return peer, true
	}

	// Walk X-Forwarded-For right to left; entries left of the first untrusted
	// hop were supplied by the client and can't be trusted.
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		addr = addr.Unmap()
		if !containsAddr(trustedProxies, addr) {
			return addr, true
		}
	}
	// Every hop is a proxy of ours: the request didn't come from outside.
	return peer, true
}

// parsePrefixes parses comma-separated IPs ("1.2.3.4") and CIDRs ("1.2.3.0/24").
func parsePrefixes(list string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			p, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, err
			}
			out = append(out, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(item)
		if err != nil {
			return nil, err
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}

func containsAddr(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
