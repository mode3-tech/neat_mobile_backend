package auth

import (
	"context"
	"errors"
	"testing"

	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/providers/bvn"
)

type stubProviderSource struct {
	provider Provider
	err      error
}

func (s stubProviderSource) GetCurrentProvider(context.Context) (Provider, error) {
	if s.err != nil {
		return "", s.err
	}

	return s.provider, nil
}

type stubTendarValidation struct {
	called bool
	resp   *bvn.TendarBVNValidationSuccessResponse
	err    error
}

func (s *stubTendarValidation) ValidateBVNWithTendar(context.Context, string) (*bvn.TendarBVNValidationSuccessResponse, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}

	return s.resp, nil
}

type stubPremblyValidation struct {
	called bool
}

func (s *stubPremblyValidation) ValidateBVNWithPrembly(context.Context, string) (*bvn.PremblyBVNValidationSuccessResponse, error) {
	s.called = true
	return nil, nil
}

func TestService_ValidateBVN_UsesCurrentProviderFromSource(t *testing.T) {
	tendarValidator := &stubTendarValidation{
		resp: &bvn.TendarBVNValidationSuccessResponse{
			Data: bvn.TendarBVNValidationSuccessData{
				Details: bvn.TendarBVNDetails{
					FirstName:   "jane",
					MiddleName:  "mary",
					LastName:    "doe",
					DateOfBirth: "1994-01-02",
					PhoneNumber: "08012345678",
				},
			},
		},
	}
	premblyValidator := &stubPremblyValidation{}
	service := NewService(
		nil,
		&verification.VerificationRepo{},
		nil,
		tendarValidator,
		premblyValidator,
		nil,
		stubProviderSource{provider: ProviderTendar},
	)

	got, err := service.ValidateBVN(context.Background(), "12345678901")
	if err != nil {
		t.Fatalf("ValidateBVN returned error: %v", err)
	}
	if got == nil {
		t.Fatal("ValidateBVN returned nil info")
	}
	if !tendarValidator.called {
		t.Fatal("expected tendar validator to be called")
	}
	if premblyValidator.called {
		t.Fatal("did not expect prembly validator to be called")
	}
	if got.name != "Jane Mary Doe" {
		t.Fatalf("unexpected name: got %q", got.name)
	}
	if got.dob != "1994-01-02" {
		t.Fatalf("unexpected dob: got %q", got.dob)
	}
	if got.phone != "08012345678" {
		t.Fatalf("unexpected phone: got %q", got.phone)
	}
}

func TestService_ValidateBVN_ReturnsProviderSourceError(t *testing.T) {
	wantErr := errors.New("cba unavailable")
	service := NewService(nil, nil, nil, nil, nil, nil, stubProviderSource{err: wantErr})

	_, err := service.ValidateBVN(context.Background(), "12345678901")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}
