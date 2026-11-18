package registerv2

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"

	"github.com/gin-gonic/gin"
)

func TestWriteRegistration_StatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := map[string]int{
		RegistrationStatusCompleted: http.StatusOK,
		RegistrationStatusPending:   http.StatusAccepted,
		RegistrationStatusFailed:    http.StatusOK,
		RegistrationStatusExpired:   http.StatusOK,
	}
	for status, want := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		(&Handler{}).writeRegistration(c, &RegisterResponse{Status: status, Message: "m"})
		if w.Code != want {
			t.Errorf("%s: code = %d, want %d", status, w.Code, want)
		}
	}
}

func TestWriteError_DoesNotLeakInternals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name     string
		err      error
		status   int
		code     string
		contains string
		hides    string
	}{
		{"internal db error", errors.New(`pq: relation "wallet_users" does not exist`), http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong", "pq:"},
		{"user-facing", userErrorf("phone number is not verified"), http.StatusUnprocessableEntity, "REGISTRATION_VALIDATION_FAILED", "phone number is not verified", ""},
		{"alat unreachable", fmt.Errorf("wrap: %w", &appErr.AlatError{Message: "alat request failed", NoResponse: true}), http.StatusServiceUnavailable, "PROVIDER_UNAVAILABLE", "temporarily unavailable", "alat request failed"},
		{"alat answer", &appErr.AlatError{Code: "InvalidNin", Message: "NIN is invalid"}, http.StatusUnprocessableEntity, "InvalidNin", "NIN is invalid", ""},
		{"known app error", appErr.ErrUserExists, response.MapError(appErr.ErrUserExists).Status, response.MapError(appErr.ErrUserExists).Error.Code, "", ""},
		{"otp error", otpError(errors.New("invalid otp")), response.MapError(appErr.ErrInvalidOTP).Status, response.MapError(appErr.ErrInvalidOTP).Error.Code, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/auth/register", nil)
			(&Handler{}).writeError(c, tc.err)
			body := w.Body.String()
			if w.Code != tc.status || !strings.Contains(body, `"code":"`+tc.code+`"`) {
				t.Fatalf("got %d %s; want %d code %s", w.Code, body, tc.status, tc.code)
			}
			if tc.contains != "" && !strings.Contains(body, tc.contains) {
				t.Errorf("body %s should contain %q", body, tc.contains)
			}
			if tc.hides != "" && strings.Contains(body, tc.hides) {
				t.Errorf("body %s leaks %q", body, tc.hides)
			}
		})
	}
}
