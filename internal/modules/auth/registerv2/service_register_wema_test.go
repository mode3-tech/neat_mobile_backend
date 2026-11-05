package registerv2

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	appErr "neat_mobile_app_backend/internal/errors"
	phoneutil "neat_mobile_app_backend/internal/phone"
)

// These drive Register's wema branch up to (but not past) the provider
// calls, against sqlmock. Every scenario here ends before the DB transaction
// that creates the user, so no Transactor/cipher is needed.

const (
	testBVN       = "12345678901"
	testNIN       = "10987654321"
	testPhone     = "08012345678"
	testEmail     = "test@example.com"
	testPhoneVer  = "phone-ver-1"
	testEmailVer  = "email-ver-1"
	testFaceCheck = "face-1"
)

var faceCheckCols = []string{"id", "verification_record_id", "matched", "face_image_provided"}

func wemaRegisterRequest() OptimusRegisterRequest {
	biometrics := false
	return OptimusRegisterRequest{
		Password:                  "Str0ng!Pass",
		ConfirmPassword:           "Str0ng!Pass",
		TransactionPin:            "1357",
		ConfirmTransactionPin:     "1357",
		PhoneNumber:               testPhone,
		Email:                     testEmail,
		PhoneVerificationID:       testPhoneVer,
		EmailVerificationID:       testEmailVer,
		BVN:                       testBVN,
		NIN:                       testNIN,
		FirstName:                 "Test",
		LastName:                  "User",
		Dob:                       "1990-01-01",
		IsBiomtricsEnabled:        &biometrics,
		BVNWithFaceVerificationID: testFaceCheck,
	}
}

func newWemaService(t *testing.T) (*Service, sqlmock.Sqlmock, *mockAlatValidator) {
	t.Helper()
	repo, mock := newSQLMockRepo(t)
	alat := &mockAlatValidator{}
	return &Service{
		repo:               repo,
		providerPreference: &mockProviderPref{provider: "wema"},
		alat:               alat,
	}, mock, alat
}

// expectRegisterPreamble covers the phone/email verification lookups and the
// duplicate-user checks that run before the provider is chosen.
func expectRegisterPreamble(t *testing.T, mock sqlmock.Sqlmock, identityMatches int) {
	t.Helper()
	phone, err := phoneutil.NormalizeNigerianNumber(testPhone)
	if err != nil {
		t.Fatalf("normalize phone: %v", err)
	}
	expectVerificationByID(mock, testPhoneVer, []driver.Value{testPhoneVer, "phone", "verified", "h", phone, nil, nil})
	expectVerificationByID(mock, testEmailVer, []driver.Value{testEmailVer, "email", "verified", "h", nil, testEmail, nil})
	mock.ExpectQuery(`SELECT id,phone,password_hash,closed_at,created_at FROM "wallet_users" WHERE phone = \$1`).
		WithArgs(phone, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "wallet_users" WHERE bvn_hash = \$1 OR nin_hash = \$2`).
		WithArgs(bvnSubjectHash(testBVN), bvnSubjectHash(testNIN)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(identityMatches))
}

func expectFaceCheckByID(mock sqlmock.Sqlmock, id, verificationID string, matched bool) {
	mock.ExpectQuery(`SELECT \* FROM "wallet_face_check_records" WHERE id = \$1`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows(faceCheckCols).AddRow(id, verificationID, matched, "live-image-b64"))
}

func expectLatestBVNRecord(mock sqlmock.Sqlmock, row []driver.Value) {
	rows := sqlmock.NewRows(verificationCols)
	if row != nil {
		rows.AddRow(row...)
	}
	mock.ExpectQuery(`SELECT \* FROM "wallet_verification_records" WHERE type = \$1 AND subject_hash = \$2 ORDER BY created_at DESC LIMIT \$3`).
		WithArgs("bvn", bvnSubjectHash(testBVN), 1).
		WillReturnRows(rows)
}

func expectNoFaceCheckForVerification(mock sqlmock.Sqlmock, verificationID string) {
	mock.ExpectQuery(`SELECT \* FROM "wallet_face_check_records" WHERE verification_record_id = \$1`).
		WithArgs(verificationID, 1).
		WillReturnRows(sqlmock.NewRows(faceCheckCols))
}

func verifiedBVNRow(id string) []driver.Value {
	return []driver.Value{id, "bvn", "verified", bvnSubjectHash(testBVN), nil, nil, nil}
}

// expectMatchedFaceForBVN sets up the happy path through face resolution: the
// requested face check points at bvnID, which is a valid verified record.
func expectMatchedFaceForBVN(mock sqlmock.Sqlmock, bvnID string) {
	expectFaceCheckByID(mock, testFaceCheck, bvnID, true)
	expectVerificationByID(mock, bvnID, verifiedBVNRow(bvnID))
}

func assertNoAccountCreated(t *testing.T, alat *mockAlatValidator) {
	t.Helper()
	if alat.tier2Calls != 0 {
		t.Fatalf("expected no Tier 2 account creation, got %d calls", alat.tier2Calls)
	}
	if alat.pndCalls != 0 {
		t.Fatalf("expected no PND calls, got %d", alat.pndCalls)
	}
}

func TestRegisterWema_DuplicateBVNRejectedBeforeProviderCalls(t *testing.T) {
	s, mock, alat := newWemaService(t)
	expectRegisterPreamble(t, mock, 1)

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if !errors.Is(err, appErr.ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
	if alat.getStep3Calls != 0 {
		t.Fatal("no ALAT call should happen for a duplicate BVN/NIN")
	}
	assertNoAccountCreated(t, alat)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRegisterWema_ExistingAccountWithDifferentDetailsRejected(t *testing.T) {
	cases := []struct {
		name    string
		details AlatAccountDetails
	}{
		{"email", AlatAccountDetails{AccountNumber: "0123456789", FirstName: "Test", LastName: "User", Email: "someone.else@example.com"}},
		{"first name", AlatAccountDetails{AccountNumber: "0123456789", FirstName: "Other", LastName: "User", Email: testEmail}},
		{"last name", AlatAccountDetails{AccountNumber: "0123456789", FirstName: "Test", LastName: "Person", Email: testEmail}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, mock, alat := newWemaService(t)
			details := tc.details
			alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) { return &details, nil }
			// If the identity check ever regresses, fail at PND rather than
			// running on into the DB transaction this harness doesn't set up.
			alat.pndStep4Func = func(ctx context.Context, pndType, accountNumber string) error { return errors.New("reuse attempted") }

			expectRegisterPreamble(t, mock, 0)
			expectMatchedFaceForBVN(mock, "bvn-1")

			_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
			if err == nil || !strings.Contains(err.Error(), "already exists for this phone under different details") {
				t.Fatalf("expected mismatch error, got %v", err)
			}
			assertNoAccountCreated(t, alat)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sql expectations: %v", err)
			}
		})
	}
}

func TestAlatAccountMatchesApplicant_IgnoresCaseWhitespaceAndEmptyFields(t *testing.T) {
	d := &AlatAccountDetails{AccountNumber: "1", FirstName: "  TEST ", LastName: "user", Email: "Test@Example.com"}
	if !alatAccountMatchesApplicant(d, testEmail, "Test", " User ") {
		t.Fatal("expected case/whitespace-insensitive match")
	}
	if !alatAccountMatchesApplicant(&AlatAccountDetails{AccountNumber: "1"}, testEmail, "Test", "User") {
		t.Fatal("fields ALAT leaves empty should not cause a mismatch")
	}
}

// Reusing an existing Wema account still requires a matched face check.
func TestRegisterWema_ExistingAccountWithoutMatchedFaceRejected(t *testing.T) {
	s, mock, alat := newWemaService(t)

	expectRegisterPreamble(t, mock, 0)
	expectFaceCheckByID(mock, testFaceCheck, "bvn-1", false) // not matched
	expectVerificationByID(mock, "bvn-1", verifiedBVNRow("bvn-1"))
	expectNoFaceCheckForVerification(mock, "bvn-1")

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "face biometric verification is required") {
		t.Fatalf("expected face verification error, got %v", err)
	}
	if alat.getStep3Calls != 0 {
		t.Fatal("ALAT account lookup must not happen before the face check passes")
	}
	assertNoAccountCreated(t, alat)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// The face check points at an older verified record for the same BVN (the
// user ran /validate/bvn twice). Register must use that record rather than
// the latest one, and get past the face check to the ALAT lookup.
func TestRegisterWema_FaceCheckOnOlderVerifiedRecordAccepted(t *testing.T) {
	s, mock, alat := newWemaService(t)
	reuse := &AlatAccountDetails{AccountNumber: "0123456789", FirstName: "Test", LastName: "User", Email: testEmail}
	alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) { return reuse, nil }
	// Stop right after the face check + identity check: failing PND ends the
	// call before the DB transaction this harness doesn't set up.
	alat.pndStep4Func = func(ctx context.Context, pndType, accountNumber string) error { return errors.New("stop here") }

	expectRegisterPreamble(t, mock, 0)
	expectMatchedFaceForBVN(mock, "bvn-older")
	// No latest-by-hash query and no face-by-verification query expected: the
	// face's own record is used directly.

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "failed to lift debit restriction") {
		t.Fatalf("expected to reach the PND step, got %v", err)
	}
	if alat.getStep3Calls != 1 {
		t.Fatalf("expected the ALAT account lookup to run once, got %d", alat.getStep3Calls)
	}
	if alat.tier2Calls != 0 {
		t.Fatal("matching existing account should be reused, not re-created")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRegisterWema_ExpiredOrUsedBVNRecordGivesSpecificError(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name    string
		row     []driver.Value
		wantErr string
	}{
		{"expired", []driver.Value{"bvn-1", "bvn", "verified", bvnSubjectHash(testBVN), nil, nil, past}, "bvn verification has expired, please validate again"},
		{"used", []driver.Value{"bvn-1", "bvn", "used", bvnSubjectHash(testBVN), nil, nil, nil}, "this bvn has already been used for registration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, mock, alat := newWemaService(t)

			expectRegisterPreamble(t, mock, 0)
			// Face check's own record is ineligible, so Register falls back to
			// the latest record, which is the same one.
			expectFaceCheckByID(mock, testFaceCheck, "bvn-1", true)
			expectVerificationByID(mock, "bvn-1", tc.row)
			expectLatestBVNRecord(mock, tc.row)

			_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("expected %q, got %v", tc.wantErr, err)
			}
			if alat.getStep3Calls != 0 {
				t.Fatal("no ALAT call expected")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sql expectations: %v", err)
			}
		})
	}
}

func TestRegisterWema_TransientAccountLookupErrorDoesNotCreateAccount(t *testing.T) {
	s, mock, alat := newWemaService(t)
	alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) {
		return nil, errors.New("alat request failed: dial tcp: i/o timeout")
	}

	expectRegisterPreamble(t, mock, 0)
	expectMatchedFaceForBVN(mock, "bvn-1")

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Fatalf("expected temporarily-unavailable error, got %v", err)
	}
	assertNoAccountCreated(t, alat)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// ALAT's own error envelope (e.g. "account not found") does mean "no account
// yet", so creation proceeds.
func TestRegisterWema_AlatErrorOnLookupProceedsToCreation(t *testing.T) {
	s, mock, alat := newWemaService(t)
	alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) {
		return nil, &appErr.AlatError{StatusCode: "404", Message: "account not found"}
	}
	alat.tier2Func = func(ctx context.Context, req AlatTier2FaceRequest) (string, error) {
		if req.LiveImageOfFace != "live-image-b64" {
			t.Errorf("expected live image from the face check record, got %q", req.LiveImageOfFace)
		}
		return "", errors.New("stop here")
	}

	expectRegisterPreamble(t, mock, 0)
	expectMatchedFaceForBVN(mock, "bvn-1")

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "tier 2 account creation failed") {
		t.Fatalf("expected to reach Tier 2 creation, got %v", err)
	}
	if alat.tier2Calls != 1 {
		t.Fatalf("expected one Tier 2 call, got %d", alat.tier2Calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
