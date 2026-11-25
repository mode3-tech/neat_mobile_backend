package registerv2

import (
	"context"
	"database/sql/driver"
	"errors"
	"neat_mobile_app_backend/models"
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
		lockOverride:       func(ctx context.Context, keys ...string) (func(), bool, error) { return func() {}, true, nil },
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

func verifiedBVNRow(id string) []driver.Value {
	return []driver.Value{id, "bvn", "verified", bvnSubjectHash(testBVN), nil, nil, nil}
}

const testWemaCorrelationID = "wema-c-id-1"

var wemaFaceCols = []string{"id", "id_type", "subject_hash", "correlation_id"}

// expectWemaFaceVerification: Wema's face web app reported a correlationId for
// the applicant's BVN.
func expectWemaFaceVerification(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "wallet_wema_face_verifications" WHERE id_type = \$1 AND subject_hash = \$2 AND expires_at > \$3`).
		WithArgs("bvn", wemaFaceSubjectHash(testBVN), sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows(wemaFaceCols).AddRow("wf-1", "bvn", wemaFaceSubjectHash(testBVN), testWemaCorrelationID))
}

// expectNoWemaFaceVerification: nothing reported for the BVN or the NIN.
func expectNoWemaFaceVerification(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "wallet_wema_face_verifications" WHERE id_type = \$1 AND subject_hash = \$2`).
		WithArgs("bvn", wemaFaceSubjectHash(testBVN), sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows(wemaFaceCols))
	mock.ExpectQuery(`SELECT \* FROM "wallet_wema_face_verifications" WHERE id_type = \$1 AND subject_hash = \$2`).
		WithArgs("nin", wemaFaceSubjectHash(testNIN), sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows(wemaFaceCols))
}

// expectMatchedFaceForBVN sets up the happy path through the face step: the
// requested face check resolves bvnID (a valid verified record), and Wema has
// reported a correlationId for the BVN.
func expectMatchedFaceForBVN(mock sqlmock.Sqlmock, bvnID string) {
	expectFaceCheckByID(mock, testFaceCheck, bvnID, true)
	expectVerificationByID(mock, bvnID, verifiedBVNRow(bvnID))
	expectWemaFaceVerification(mock)
}

// expectNoPendingWemaRegistration: no earlier attempt is waiting on ALAT for
// this phone.
func expectNoPendingWemaRegistration(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "wallet_pending_wema_registrations" WHERE phone = \$1 AND status IN \(\$2,\$3\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
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
			expectNoPendingWemaRegistration(mock)

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
	if !alatAccountMatchesApplicant(d, nil, "Test", " User ") {
		t.Fatal("expected case/whitespace-insensitive match")
	}
	if !alatAccountMatchesApplicant(&AlatAccountDetails{AccountNumber: "1"}, nil, "Test", "User") {
		t.Fatal("fields ALAT leaves empty should not cause a mismatch")
	}
}

func bvnRecordNamed(name, dob string) *models.VerificationRecord {
	return &models.VerificationRecord{ID: "bvn-1", VerifiedName: &name, VerifiedDOB: &dob}
}

// ALAT names the account from the BVN, so a retry is matched against the BVN
// name, not the typed one - a nickname or a new email must not strand it.
func TestAlatAccountMatchesApplicant_UsesBVNName(t *testing.T) {
	bvn := bvnRecordNamed("Adebayo Tunde Ogunleye", "1990-01-01")
	acct := &AlatAccountDetails{AccountNumber: "1", FirstName: "ADEBAYO", LastName: "OGUNLEYE", Email: "old@example.com"}
	if !alatAccountMatchesApplicant(acct, bvn, "Bayo", "Ogunleye") {
		t.Fatal("account named from the BVN should match regardless of the typed name or email")
	}
	other := &AlatAccountDetails{AccountNumber: "1", FirstName: "Chidi", LastName: "Okafor"}
	if alatAccountMatchesApplicant(other, bvn, "Chidi", "Okafor") {
		t.Fatal("an account under someone else's name must not match, even if the typed name does")
	}
}

func TestCheckApplicantMatchesBVN(t *testing.T) {
	bvn := bvnRecordNamed("Adebayo Tunde Ogunleye", "01-Jan-1990")
	cases := []struct {
		name, first, last, dob string
		wantErr                bool
	}{
		{"exact", "Adebayo", "Ogunleye", "1990-01-01", false},
		{"case, order and punctuation", "ogunleye", "ADEBAYO-", "1990-01-01", false},
		{"middle name as first", "Tunde", "Ogunleye", "1990-01-01", false},
		{"nickname", "Bayo", "Ogunleye", "1990-01-01", true},
		{"wrong surname", "Adebayo", "Okafor", "1990-01-01", true},
		{"wrong dob", "Adebayo", "Ogunleye", "1991-01-01", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkApplicantMatchesBVN(bvn, tc.first, tc.last, tc.dob)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
	// Records without name/DOB (e.g. Optimus) and no record skip the check.
	if err := checkApplicantMatchesBVN(&models.VerificationRecord{ID: "x"}, "Any", "One", "2000-01-01"); err != nil {
		t.Fatalf("record without name/dob: %v", err)
	}
	if err := checkApplicantMatchesBVN(nil, "Any", "One", "2000-01-01"); err != nil {
		t.Fatalf("nil record: %v", err)
	}
}

// Reusing an existing Wema account still requires Wema's own face check.
func TestRegisterWema_ExistingAccountWithoutWemaFaceVerificationRejected(t *testing.T) {
	s, mock, alat := newWemaService(t)

	expectRegisterPreamble(t, mock, 0)
	expectFaceCheckByID(mock, testFaceCheck, "bvn-1", true)
	expectVerificationByID(mock, "bvn-1", verifiedBVNRow("bvn-1"))
	expectNoWemaFaceVerification(mock)

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "face biometric verification with wema is required") {
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
	expectNoPendingWemaRegistration(mock)
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
	expectNoPendingWemaRegistration(mock)

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
		if req.CorrelationID != testWemaCorrelationID {
			t.Errorf("expected wema's correlationId, got %q", req.CorrelationID)
		}
		if req.LiveImageOfFace != "" {
			t.Errorf("no selfie should be sent, got %d chars", len(req.LiveImageOfFace))
		}
		return "", errors.New("stop here")
	}

	expectRegisterPreamble(t, mock, 0)
	expectMatchedFaceForBVN(mock, "bvn-1")
	expectNoPendingWemaRegistration(mock)

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

// A name or DOB that differs from the BVN stops registration before any ALAT
// call, so no account is created for a mismatched profile.
func TestRegisterWema_NameMismatchWithBVNRejectedBeforeProviderCalls(t *testing.T) {
	s, mock, alat := newWemaService(t)
	expectRegisterPreamble(t, mock, 0)

	cols := append(append([]string{}, verificationCols...), "verified_name", "verified_dob")
	row := append(verifiedBVNRow("bvn-1"), "Chidi Okafor", "01-01-1990")
	expectFaceCheckByID(mock, testFaceCheck, "bvn-1", true)
	mock.ExpectQuery(`SELECT \* FROM "wallet_verification_records" WHERE id = \$1 LIMIT \$2`).
		WithArgs("bvn-1", 1).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(row...))

	_, err := s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "must match the name on your BVN") {
		t.Fatalf("expected BVN name mismatch, got %v", err)
	}
	if alat.getStep3Calls != 0 {
		t.Fatal("no ALAT call should happen when the applicant doesn't match the BVN")
	}
	assertNoAccountCreated(t, alat)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
