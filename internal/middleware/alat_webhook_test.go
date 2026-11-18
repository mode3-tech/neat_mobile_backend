package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAlatWebhookIPAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name           string
		allowed        string
		trustedProxies string
		remoteAddr     string
		xff            string
		want           int
	}{
		{"allowed peer", "41.58.1.10", "", "41.58.1.10:5000", "", http.StatusOK},
		{"allowed by CIDR", "41.58.1.0/24", "", "41.58.1.77:5000", "", http.StatusOK},
		{"unknown peer", "41.58.1.10", "", "8.8.8.8:5000", "", http.StatusForbidden},
		{"XFF ignored from untrusted peer", "41.58.1.10", "", "8.8.8.8:5000", "41.58.1.10", http.StatusForbidden},
		{"allowed via trusted proxy", "41.58.1.10", "10.0.0.0/8", "10.0.0.5:5000", "41.58.1.10", http.StatusOK},
		{"spoofed left XFF entry", "41.58.1.10", "10.0.0.0/8", "10.0.0.5:5000", "41.58.1.10, 8.8.8.8", http.StatusForbidden},
		{"multiple proxy hops", "41.58.1.10", "10.0.0.0/8", "10.0.0.5:5000", "41.58.1.10, 10.0.0.9", http.StatusOK},
		{"empty allowlist fails closed", "", "", "41.58.1.10:5000", "", http.StatusForbidden},
		{"invalid allowlist fails closed", "not-an-ip", "", "41.58.1.10:5000", "", http.StatusForbidden},
		{"IPv6 peer", "2001:db8::1", "", "[2001:db8::1]:5000", "", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/hook", AlatWebhookIPAllowlist(tc.allowed, tc.trustedProxies), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodPost, "/hook", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
