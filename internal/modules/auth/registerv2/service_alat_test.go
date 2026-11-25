package registerv2

import (
	"context"
	"database/sql/driver"
	"errors"
	"neat_mobile_app_backend/internal/modules/auth"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type mockProviderPref struct {
	provider string
}

func (m *mockProviderPref) GetProviderPreference(ctx context.Context) (*ProviderPreference, error) {
	return &ProviderPreference{Provider: m.provider}, nil
}

type mockAlatValidator struct {
	getStep3Func func(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error)
	pndStep4Func func(ctx context.Context, pndType, accountNumber string) error
	tier2Func    func(ctx context.Context, req AlatTier2FaceRequest) (string, error)
	tier2Message string // ALAT's message returned with the tier 2 result

	getStep3Calls int
	tier2Calls    int
	pndCalls      int
}

func (m *mockAlatValidator) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error) {
	m.getStep3Calls++
	if m.getStep3Func != nil {
		return m.getStep3Func(ctx, phoneNumber)
	}
	return &AlatAccountDetails{
		AccountNumber: "0123456789",
		FirstName:     "Test",
		LastName:      "User",
		Email:         "test@example.com",
		PhoneNumber:   phoneNumber,
	}, nil
}

func (m *mockAlatValidator) PartnerDebitRestrictionManagement(ctx context.Context, pndType, accountNumber string) error {
	m.pndCalls++
	if m.pndStep4Func != nil {
		return m.pndStep4Func(ctx, pndType, accountNumber)
	}
	return nil
}

func (m *mockAlatValidator) PostTier2PartnershipAccountWithFace(ctx context.Context, req AlatTier2FaceRequest) (AlatTier2Result, error) {
	m.tier2Calls++
	if m.tier2Func != nil {
		id, err := m.tier2Func(ctx, req)
		return AlatTier2Result{TrackingID: id, Message: m.tier2Message}, err
	}
	return AlatTier2Result{TrackingID: "mock-tracking-face-123", Message: m.tier2Message}, nil
}

func (m *mockAlatValidator) GetAddressDropDownList(ctx context.Context) (any, error) {
	return map[string]any{}, nil
}

// Wema registration validates the BVN through the KYC provider (no OTP); it
// never falls back to ALAT's BVN + OTP account creation API.
func TestValidateBVN_Wema_RequiresKYCProvider(t *testing.T) {
	s := &Service{
		providerPreference: &mockProviderPref{provider: "wema"},
		alat:               &mockAlatValidator{},
	}
	_, ref, requiresOTP, err := s.ValidateBVN(context.Background(), OptimusBVNValidationRequest{
		Bvn:         "12345678901",
		Email:       "test@example.com",
		PhoneNumber: "08012345678",
	})
	if err == nil || ref != "" || requiresOTP {
		t.Fatalf("got ref=%q requiresOTP=%v err=%v; want a not-configured error and no OTP", ref, requiresOTP, err)
	}
}

func TestOTP_Wema_Rejected(t *testing.T) {
	for _, provider := range []string{"wema", "alat"} {
		s := &Service{
			providerPreference: &mockProviderPref{provider: provider},
			alat:               &mockAlatValidator{},
		}
		if err := s.VerifyOTP(context.Background(), "08012345678", "123456", "test@example.com", "ref-1"); !errors.Is(err, errWemaNoOTP) {
			t.Errorf("%s VerifyOTP err = %v; want errWemaNoOTP", provider, err)
		}
		if _, err := s.ResendOTP(context.Background(), "ref-1"); !errors.Is(err, errWemaNoOTP) {
			t.Errorf("%s ResendOTP err = %v; want errWemaNoOTP", provider, err)
		}
	}
}

// An OTP challenge is routed by the provider on its verification record, not
// the current preference; a Wema record is rejected since Wema has no OTP.
func TestOTP_UsesRecordProviderEvenIfPreferenceChanged(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer sqlDB.Close()
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	s := &Service{
		repo:               NewRepository(gormDB, nil),
		providerPreference: &mockProviderPref{provider: "optimus"}, // preference changed to optimus
		alat:               &mockAlatValidator{},
	}

	for _, ref := range []string{"tracking-ref-123", "tracking-ref-456"} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "wallet_verification_records" WHERE provider_verification_id = $1 ORDER BY created_at DESC LIMIT $2`)).
			WithArgs(ref, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_verification_id", "status"}).
				AddRow("rec-"+ref, "wema", ref, "pending"))
	}

	if err := s.VerifyOTP(context.Background(), "08012345678", "123456", "test@example.com", "tracking-ref-123"); !errors.Is(err, errWemaNoOTP) {
		t.Fatalf("VerifyOTP err = %v; want errWemaNoOTP (record provider wema)", err)
	}
	if _, err := s.ResendOTP(context.Background(), "tracking-ref-456"); !errors.Is(err, errWemaNoOTP) {
		t.Fatalf("ResendOTP err = %v; want errWemaNoOTP (record provider wema)", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

type stubSessionIssuer struct {
	validateBVNWithFaceFunc func(ctx context.Context, payload auth.BVNWithFaceValidationRequest) (*auth.BVNWithFaceInfo, error)
}

func (s *stubSessionIssuer) IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error) {
	return nil, nil
}

func (s *stubSessionIssuer) ValidateBVN(ctx context.Context, bvn string) (*auth.BVNInfo, error) {
	return nil, nil
}

func (s *stubSessionIssuer) ValidateBVNWithFace(ctx context.Context, payload auth.BVNWithFaceValidationRequest) (*auth.BVNWithFaceInfo, error) {
	if s.validateBVNWithFaceFunc != nil {
		return s.validateBVNWithFaceFunc(ctx, payload)
	}
	return &auth.BVNWithFaceInfo{FaceCheckID: "face-check-123"}, nil
}

func newSQLMockRepo(t *testing.T) (*Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	return NewRepository(gormDB, nil), mock
}

var verificationCols = []string{"id", "type", "status", "subject_hash", "verified_phone", "verified_email", "expires_at"}

func expectVerificationByID(mock sqlmock.Sqlmock, id string, row []driver.Value) {
	rows := sqlmock.NewRows(verificationCols)
	if row != nil {
		rows.AddRow(row...)
	}
	mock.ExpectQuery(`SELECT \* FROM "wallet_verification_records" WHERE id = \$1 LIMIT \$2`).
		WithArgs(id, 1).
		WillReturnRows(rows)
}

func TestValidateBVNWithFace_PendingRecordIsMarkedVerified(t *testing.T) {
	repo, mock := newSQLMockRepo(t)
	s := &Service{repo: repo, sessionIssuer: &stubSessionIssuer{}}

	expectVerificationByID(mock, "ver-123", []driver.Value{"ver-123", "bvn", "pending", bvnSubjectHash(testBVN), nil, nil, nil})
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "wallet_verification_records" SET .+ WHERE id = \$4 AND type = \$5 AND status = \$6 AND \(expires_at IS NULL OR expires_at > \$7\)`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "ver-123", "bvn", "pending", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	faceCheckID, err := s.ValidateBVNWithFace(context.Background(), BVNWithFaceRequest{
		VerificationID: "ver-123",
		BVN:            testBVN,
		Image:          "base64image",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if faceCheckID != "face-check-123" {
		t.Fatalf("expected faceCheckID 'face-check-123', got '%s'", faceCheckID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// Tendar/Prembly /validate/bvn saves records as verified straight away - the
// face step must accept that and must not try to re-mark it (which would fail,
// as MarkVerificationVerified only flips pending records).
func TestValidateBVNWithFace_AlreadyVerifiedRecordSucceedsWithoutUpdate(t *testing.T) {
	repo, mock := newSQLMockRepo(t)
	s := &Service{repo: repo, sessionIssuer: &stubSessionIssuer{}}

	expectVerificationByID(mock, "ver-123", []driver.Value{"ver-123", "bvn", "verified", bvnSubjectHash(testBVN), nil, nil, nil})

	faceCheckID, err := s.ValidateBVNWithFace(context.Background(), BVNWithFaceRequest{
		VerificationID: "ver-123",
		BVN:            testBVN,
		Image:          "base64image",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if faceCheckID != "face-check-123" {
		t.Fatalf("expected faceCheckID 'face-check-123', got '%s'", faceCheckID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestValidateBVNWithFace_RejectsIneligibleRecords(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name    string
		row     []driver.Value
		wantErr string
	}{
		{"missing", nil, "not found"},
		{"wrong type", []driver.Value{"ver-123", "nin", "verified", bvnSubjectHash(testBVN), nil, nil, nil}, "not found"},
		{"used", []driver.Value{"ver-123", "bvn", "used", bvnSubjectHash(testBVN), nil, nil, nil}, "already been used"},
		{"failed", []driver.Value{"ver-123", "bvn", "failed", bvnSubjectHash(testBVN), nil, nil, nil}, "verification failed"},
		{"expired", []driver.Value{"ver-123", "bvn", "verified", bvnSubjectHash(testBVN), nil, nil, past}, "expired"},
		{"other bvn", []driver.Value{"ver-123", "bvn", "verified", bvnSubjectHash("99999999999"), nil, nil, nil}, "does not match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newSQLMockRepo(t)
			issuerCalled := false
			s := &Service{repo: repo, sessionIssuer: &stubSessionIssuer{
				validateBVNWithFaceFunc: func(ctx context.Context, payload auth.BVNWithFaceValidationRequest) (*auth.BVNWithFaceInfo, error) {
					issuerCalled = true
					return &auth.BVNWithFaceInfo{FaceCheckID: "x"}, nil
				},
			}}
			expectVerificationByID(mock, "ver-123", tc.row)

			_, err := s.ValidateBVNWithFace(context.Background(), BVNWithFaceRequest{VerificationID: "ver-123", BVN: testBVN, Image: "img"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if issuerCalled {
				t.Fatal("face provider must not be called for an ineligible record")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sql expectations: %v", err)
			}
		})
	}
}

func TestGetAddressDropDownList_Success(t *testing.T) {
	mockAlat := &mockAlatValidator{}
	s := &Service{alat: mockAlat}

	data, err := s.GetAddressDropDownList(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatalf("expected non-nil data")
	}
}
