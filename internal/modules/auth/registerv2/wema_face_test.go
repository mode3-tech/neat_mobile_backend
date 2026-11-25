package registerv2

import (
	"context"
	"database/sql/driver"
	"net/url"
	"strings"
	"testing"
	"time"

	"neat_mobile_app_backend/models"

	"github.com/DATA-DOG/go-sqlmock"
)

const testFaceCallback = "https://api.example.com/api/v1/alat/facial-verification"

func TestWemaFaceURL(t *testing.T) {
	s := &Service{}
	s.SetWemaFaceConfig("https://face.example.net/", "key-123", testFaceCallback)

	got, err := s.WemaFaceURL(" BVN ", "12345678901")
	if err != nil {
		t.Fatalf("WemaFaceURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	if u.Scheme+"://"+u.Host != "https://face.example.net" || u.Path != "/" {
		t.Fatalf("unexpected base in %q", got)
	}
	// Wema reports the result to the whitelisted callback; no redirect.
	q := u.Query()
	if q.Get("bvn") != "12345678901" || q.Get("x_tk") != "key-123" || q.Get("cb_uri") != testFaceCallback || q.Get("rd_uri") != "" {
		t.Fatalf("unexpected query in %q", got)
	}

	nin, err := s.WemaFaceURL("nin", "10987654321")
	if err != nil || !strings.Contains(nin, "nin=10987654321") || strings.Contains(nin, "bvn=") {
		t.Fatalf("nin url = %q, err = %v", nin, err)
	}
}

func TestWemaFaceURL_Rejects(t *testing.T) {
	configured := &Service{}
	configured.SetWemaFaceConfig("https://face.example.net", "key", testFaceCallback)

	if _, err := configured.WemaFaceURL("passport", "123"); err == nil {
		t.Fatal("unknown id_type should be rejected")
	}
	if _, err := configured.WemaFaceURL("bvn", " "); err == nil {
		t.Fatal("empty id should be rejected")
	}
	if _, err := (&Service{}).WemaFaceURL("bvn", "12345678901"); err == nil {
		t.Fatal("an unconfigured service should refuse to build a URL")
	}
	// Without the callback the flow cannot work, so no URL is handed out.
	noCallback := &Service{}
	noCallback.SetWemaFaceConfig("https://face.example.net", "key", "")
	if got, err := noCallback.WemaFaceURL("bvn", "12345678901"); err == nil {
		t.Fatalf("no callback configured: url = %q, want an error", got)
	}
}

func TestRecordWemaFaceVerification(t *testing.T) {
	t.Run("verified needs a c_id", func(t *testing.T) {
		s, _, _ := newWemaService(t)
		if err := s.RecordWemaFaceVerification(context.Background(), "bvn", testBVN, " ", true, ""); err == nil {
			t.Fatal("a successful result without a c_id should be rejected")
		}
	})
	t.Run("unsupported id_type", func(t *testing.T) {
		s, _, _ := newWemaService(t)
		if err := s.RecordWemaFaceVerification(context.Background(), "passport", testBVN, "c", true, ""); err == nil {
			t.Fatal("unknown id_type should be rejected")
		}
	})
	t.Run("failed result is stored with the reason", func(t *testing.T) {
		s, mock, _ := newWemaService(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "wallet_wema_face_verifications"`).
			WithArgs(sqlmock.AnyArg(), "bvn", wemaFaceSubjectHash(testBVN), "", "failed", "selfie error", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if err := s.RecordWemaFaceVerification(context.Background(), "bvn", testBVN, "", false, " selfie error "); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("verified result is stored", func(t *testing.T) {
		s, mock, _ := newWemaService(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "wallet_wema_face_verifications"`).
			WithArgs(sqlmock.AnyArg(), "nin", wemaFaceSubjectHash(testNIN), "c-1", "verified", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if err := s.RecordWemaFaceVerification(context.Background(), "nin", testNIN, "c-1", true, ""); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestWemaFaceStatus(t *testing.T) {
	cols := []string{"id", "id_type", "subject_hash", "correlation_id", "status", "error_message", "created_at", "expires_at"}
	query := `SELECT \* FROM "wallet_wema_face_verifications" WHERE id_type = \$1 AND subject_hash = \$2 AND expires_at > \$3 ORDER BY created_at DESC LIMIT \$4`
	row := func(status, cid, msg string) []driver.Value {
		return []driver.Value{"wf-1", "bvn", wemaFaceSubjectHash(testBVN), cid, status, msg, time.Now(), time.Now().Add(time.Hour)}
	}
	cases := []struct {
		name       string
		rows       [][]driver.Value
		wantStatus string
		wantMsg    string
	}{
		{"nothing from wema yet", nil, WemaFaceStatusPending, ""},
		{"verified", [][]driver.Value{row(models.WemaFaceStatusVerified, "c-1", "")}, WemaFaceStatusVerified, ""},
		{"failed shows wema's reason", [][]driver.Value{row(models.WemaFaceStatusFailed, "", "Oops an error occurred")}, WemaFaceStatusFailed, "Oops an error occurred"},
		{"failed without a reason", [][]driver.Value{row(models.WemaFaceStatusFailed, "", "")}, WemaFaceStatusFailed, "try again"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, mock, _ := newWemaService(t)
			rows := sqlmock.NewRows(cols)
			for _, r := range tc.rows {
				rows.AddRow(r...)
			}
			mock.ExpectQuery(query).
				WithArgs("bvn", wemaFaceSubjectHash(testBVN), sqlmock.AnyArg(), 1).
				WillReturnRows(rows)
			status, msg, err := s.WemaFaceStatus(context.Background(), "bvn", testBVN)
			if err != nil {
				t.Fatal(err)
			}
			if status != tc.wantStatus || !strings.Contains(msg, tc.wantMsg) {
				t.Fatalf("status=%q message=%q, want %q containing %q", status, msg, tc.wantStatus, tc.wantMsg)
			}
			// The c_id must never be part of what the app sees.
			if strings.Contains(msg, "c-1") {
				t.Fatalf("message leaks the c_id: %q", msg)
			}
		})
	}
	t.Run("rejects bad input", func(t *testing.T) {
		s, _, _ := newWemaService(t)
		if _, _, err := s.WemaFaceStatus(context.Background(), "passport", testBVN); err == nil {
			t.Fatal("unknown id_type should be rejected")
		}
		if _, _, err := s.WemaFaceStatus(context.Background(), "bvn", " "); err == nil {
			t.Fatal("empty id should be rejected")
		}
	})
}
