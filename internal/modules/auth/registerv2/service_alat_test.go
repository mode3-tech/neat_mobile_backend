package registerv2

import (
	"context"
	"errors"
	"testing"
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

