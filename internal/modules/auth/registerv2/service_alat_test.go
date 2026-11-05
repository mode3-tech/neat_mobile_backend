package registerv2

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/modules/auth"
	"regexp"
	"testing"

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
	postStep1Func   func(ctx context.Context, phone, email, bvn string) (string, error)
	verifyStep2Func func(ctx context.Context, phone, otp, trackingID string) error
	getStep3Func    func(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error)
	pndStep4Func    func(ctx context.Context, pndType, accountNumber string) error
	resendFunc      func(ctx context.Context, trackingID string) error
}

func (m *mockAlatValidator) PostPartnershipAccountCreationWithBvn(ctx context.Context, phone, email, bvn string) (string, error) {
	if m.postStep1Func != nil {
		return m.postStep1Func(ctx, phone, email, bvn)
	}
	return "mock-tracking-id-1234", nil
}

func (m *mockAlatValidator) ValidateBVNandEnqueueAccountCreation(ctx context.Context, phone, otp, trackingID string) error {
	if m.verifyStep2Func != nil {
		return m.verifyStep2Func(ctx, phone, otp, trackingID)
	}
	return nil
}

func (m *mockAlatValidator) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error) {
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
	if m.pndStep4Func != nil {
		return m.pndStep4Func(ctx, pndType, accountNumber)
	}
	return nil
}

func (m *mockAlatValidator) ResendOtp(ctx context.Context, trackingID string) error {
	if m.resendFunc != nil {
		return m.resendFunc(ctx, trackingID)
	}
	return nil
}

func (m *mockAlatValidator) PostTier2PartnershipAccountWithFace(ctx context.Context, req AlatTier2FaceRequest) (string, error) {
	return "mock-tracking-face-123", nil
}

func (m *mockAlatValidator) GetAddressDropDownList(ctx context.Context) (any, error) {
	return map[string]any{}, nil
}

func TestValidateBVN_Alat_RequiresPhone(t *testing.T) {
	s := &Service{
		providerPreference: &mockProviderPref{provider: "wema"},
		alat:               &mockAlatValidator{},
	}

	_, _, _, err := s.ValidateBVN(context.Background(), OptimusBVNValidationRequest{
		Bvn:   "12345678901",
		Email: "test@example.com",
		// PhoneNumber omitted
	})
	if err == nil {
		t.Fatal("expected error when phone_number is missing for ALAT")
	}
}

func TestResendOTP_Alat_Success(t *testing.T) {
	var calledTrackingID string
	mock := &mockAlatValidator{
		resendFunc: func(ctx context.Context, trackingID string) error {
			calledTrackingID = trackingID
			return nil
		},
	}

	s := &Service{
		providerPreference: &mockProviderPref{provider: "wema"},
		alat:               mock,
	}

	ref, err := s.ResendOTP(context.Background(), "tracking-uuid-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "tracking-uuid-xyz" || calledTrackingID != "tracking-uuid-xyz" {
		t.Errorf("expected tracking id tracking-uuid-xyz, got ref=%s called=%s", ref, calledTrackingID)
	}
}

func TestResendOTP_Alat_Error(t *testing.T) {
	mock := &mockAlatValidator{
		resendFunc: func(ctx context.Context, trackingID string) error {
			return errors.New("upstream alat error")
		},
	}

	s := &Service{
		providerPreference: &mockProviderPref{provider: "alat"},
		alat:               mock,
	}

	_, err := s.ResendOTP(context.Background(), "tracking-uuid-xyz")
	if err == nil {
		t.Fatal("expected error on upstream ALAT failure")
	}
}


func TestVerifyOTP_UsesRecordProviderEvenIfPreferenceChanged(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	repo := NewRepository(gormDB, nil)

	var alatCalled bool
	mockAlat := &mockAlatValidator{
		verifyStep2Func: func(ctx context.Context, phone, otp, trackingID string) error {
			alatCalled = true
			return nil
		},
	}

	s := &Service{
		repo:               repo,
		providerPreference: &mockProviderPref{provider: "optimus"}, // Preference changed to optimus!
		alat:               mockAlat,
	}

	// Expect lookup of pending verification record with provider_verification_id = "tracking-ref-123"
	rows := sqlmock.NewRows([]string{"id", "provider", "provider_verification_id", "status"}).
		AddRow("rec-1", "wema", "tracking-ref-123", "pending")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "wallet_verification_records" WHERE provider_verification_id = $1 ORDER BY created_at DESC LIMIT $2`)).
		WithArgs("tracking-ref-123", 1).
		WillReturnRows(rows)

	// Expect update to verified (GORM wraps writes in Begin/Commit)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "wallet_verification_records" SET "status"=$1,"verified_at"=$2,"updated_at"=$3 WHERE provider_verification_id = $4 AND status = $5`)).
		WithArgs("verified", sqlmock.AnyArg(), sqlmock.AnyArg(), "tracking-ref-123", "pending").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = s.VerifyOTP(context.Background(), "08012345678", "123456", "test@example.com", "tracking-ref-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alatCalled {
		t.Fatal("expected ALAT verification to be called based on DB record provider, but it was not")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestResendOTP_UsesRecordProviderEvenIfPreferenceChanged(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	repo := NewRepository(gormDB, nil)

	var alatResendCalled bool
	mockAlat := &mockAlatValidator{
		resendFunc: func(ctx context.Context, trackingID string) error {
			alatResendCalled = true
			return nil
		},
	}

	s := &Service{
		repo:               repo,
		providerPreference: &mockProviderPref{provider: "optimus"}, // Preference changed to optimus!
		alat:               mockAlat,
	}

	// Expect lookup of pending verification record with provider_verification_id = "tracking-ref-456"
	rows := sqlmock.NewRows([]string{"id", "provider", "provider_verification_id", "status"}).
		AddRow("rec-2", "wema", "tracking-ref-456", "pending")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "wallet_verification_records" WHERE provider_verification_id = $1 ORDER BY created_at DESC LIMIT $2`)).
		WithArgs("tracking-ref-456", 1).
		WillReturnRows(rows)

	ref, err := s.ResendOTP(context.Background(), "tracking-ref-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "tracking-ref-456" || !alatResendCalled {
		t.Fatalf("expected ALAT resend to be called, got ref=%s, called=%v", ref, alatResendCalled)
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

func TestValidateBVNWithFace_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	repo := NewRepository(gormDB, nil)
	stubIssuer := &stubSessionIssuer{}

	s := &Service{
		repo:          repo,
		sessionIssuer: stubIssuer,
	}

	// Expect update of verification record to verified
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "wallet_verification_records" SET .+ WHERE id = \$4 AND type = \$5 AND status = \$6 AND \(expires_at IS NULL OR expires_at > \$7\)`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "ver-123", "bvn", "pending", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	faceCheckID, err := s.ValidateBVNWithFace(context.Background(), BVNWithFaceRequest{
		VerificationID: "ver-123",
		BVN:            "12345678901",
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
