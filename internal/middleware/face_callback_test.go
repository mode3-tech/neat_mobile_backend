package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func faceCall(h gin.HandlerFunc, origin, remote string) int {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/f", h, func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/f", nil)
	req.RemoteAddr = remote
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestFaceCallbackGuard(t *testing.T) {
	const face = "https://face.example.com"
	h := FaceCallbackGuard(face, "203.0.113.0/24", "")

	if got := faceCall(h, face, "98.97.79.131:1"); got != http.StatusOK {
		t.Errorf("allowed origin: got %d", got)
	}
	if got := faceCall(h, "https://evil.example", "98.97.79.131:1"); got != http.StatusForbidden {
		t.Errorf("other origin: got %d", got)
	}
	if got := faceCall(h, "", "98.97.79.131:1"); got != http.StatusForbidden {
		t.Errorf("no origin, unlisted ip: got %d", got)
	}
	if got := faceCall(h, "", "203.0.113.9:1"); got != http.StatusOK {
		t.Errorf("allowlisted ip: got %d", got)
	}
}

func TestFaceCallbackGuardRateLimit(t *testing.T) {
	const face = "https://face.example.com"
	h := FaceCallbackGuard(face, "", "")
	for i := 0; i < faceCallbackRateLimit; i++ {
		if got := faceCall(h, face, "98.97.79.131:1"); got != http.StatusOK {
			t.Fatalf("call %d: got %d", i, got)
		}
	}
	if got := faceCall(h, face, "98.97.79.131:1"); got != http.StatusTooManyRequests {
		t.Errorf("over limit: got %d", got)
	}
}
