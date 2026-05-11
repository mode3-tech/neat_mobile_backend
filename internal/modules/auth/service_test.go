package auth

import (
	"context"
	"errors"
	"neat_mobile_app_backend/providers/bvn"
	"testing"
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
	resp   *bvn.PremblyBVNValidationSuccessResponse
	err    error
}

func (s *stubPremblyValidation) ValidateBVNWithPrembly(context.Context, string) (*bvn.PremblyBVNValidationSuccessResponse, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestService_ValidateBVN_UsesCurrentProviderFromSource(t *testing.T) {
	wantErr := errors.New("tendar invoked")
	tendarValidator := &stubTendarValidation{
		err: wantErr,
	}
	premblyValidator := &stubPremblyValidation{}
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		nil,
		tendarValidator,
		premblyValidator,
		nil,
		stubProviderSource{provider: ProviderTendar},
		nil,
		nil,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.ValidateBVN(context.Background(), "12345678901")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if !tendarValidator.called {
		t.Fatal("expected tendar validator to be called")
	}
	if premblyValidator.called {
		t.Fatal("did not expect prembly validator to be called")
	}
}

func TestService_ValidateBVN_FallsBackWhenProviderSourceFails(t *testing.T) {
	fallbackErr := errors.New("fallback validator invoked")
	tendarValidator := &stubTendarValidation{err: fallbackErr}
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		nil,
		tendarValidator,
		nil,
		nil,
		stubProviderSource{err: errors.New("cba unavailable")},
		nil,
		nil,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.ValidateBVN(context.Background(), "12345678901")
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("expected error %v, got %v", fallbackErr, err)
	}
	if !tendarValidator.called {
		t.Fatal("expected fallback validator to be called")
	}
}

func TestService_ValidateBVN_RoutesToPrembly(t *testing.T) {
	wantErr := errors.New("prembly invoked")
	tendarValidator := &stubTendarValidation{}
	premblyValidator := &stubPremblyValidation{err: wantErr}
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		nil,
		tendarValidator,
		premblyValidator,
		nil,
		stubProviderSource{provider: ProviderPrembly},
		nil,
		nil,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.ValidateBVN(context.Background(), "12345678901")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if !premblyValidator.called {
		t.Fatal("expected prembly validator to be called")
	}
	if tendarValidator.called {
		t.Fatal("did not expect tendar validator to be called")
	}
}
