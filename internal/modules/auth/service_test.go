package auth

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/providers/bvn"
	"neat_mobile_app_backend/providers/nin"
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

func (s *stubPremblyValidation) ValidateBVNWithFace(context.Context, string, string) (*bvn.PremblyBVNWithFaceResponse, error) {
	return nil, nil
}

type stubNINValidation struct {
	called bool
	resp   *nin.ValidationResponse
	err    error
}

func (s *stubNINValidation) ValidateNIN(context.Context, string) (*nin.ValidationResponse, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

// serviceDeps names the handful of collaborators the routing tests care about, so
// they don't each have to spell out all 23 positional NewService arguments.
type serviceDeps struct {
	tendar     TendarValidation
	prembly    PremblyValidation
	ninPrembly NINValidation
	ninTendar  NINValidation
	ninFace    NINFaceValidation
	source     ValidationProviderSource
}

func newTestService(deps serviceDeps) *Service {
	return NewService(
		nil, nil, nil, nil, nil, nil, nil, "", nil,
		deps.tendar,
		deps.prembly,
		deps.ninPrembly,
		deps.ninTendar,
		deps.ninFace,
		nil, // ninFaceTendar
		nil, // bvnFaceTendar
		deps.source,
		nil, nil, "", nil,
		make(chan struct{}),
		make(chan struct{}),
		"",
		0,
		"",
	)
}

func TestService_ValidateBVN_UsesCurrentProviderFromSource(t *testing.T) {
	wantErr := errors.New("tendar invoked")
	tendarValidator := &stubTendarValidation{err: wantErr}
	premblyValidator := &stubPremblyValidation{}
	service := newTestService(serviceDeps{
		tendar:  tendarValidator,
		prembly: premblyValidator,
		source:  stubProviderSource{provider: ProviderTendar},
	})

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
	service := newTestService(serviceDeps{
		tendar: tendarValidator,
		source: stubProviderSource{err: errors.New("cba unavailable")},
	})

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
	service := newTestService(serviceDeps{
		tendar:  tendarValidator,
		prembly: premblyValidator,
		source:  stubProviderSource{provider: ProviderPrembly},
	})

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

// TestService_NINProviderFor_FollowsTheSamePreferenceAsBVN is the core of this
// change: one system_preferences row moves both flows.
func TestService_NINProviderFor_FollowsTheSamePreferenceAsBVN(t *testing.T) {
	premblyClient := &stubNINValidation{}
	tendarClient := &stubNINValidation{}

	tests := []struct {
		name         string
		source       ValidationProviderSource
		wantProvider Provider
		wantClient   NINValidation
	}{
		{
			name:         "prembly preference routes to prembly",
			source:       stubProviderSource{provider: ProviderPrembly},
			wantProvider: ProviderPrembly,
			wantClient:   premblyClient,
		},
		{
			name:         "tendar preference routes to tendar",
			source:       stubProviderSource{provider: ProviderTendar},
			wantProvider: ProviderTendar,
			wantClient:   tendarClient,
		},
		{
			name:         "source failure falls back to tendar",
			source:       stubProviderSource{err: errors.New("db unavailable")},
			wantProvider: ProviderTendar,
			wantClient:   tendarClient,
		},
		{
			name:         "no source wired falls back to tendar",
			source:       nil,
			wantProvider: ProviderTendar,
			wantClient:   tendarClient,
		},
		{
			name:         "non-identity provider falls back to tendar",
			source:       stubProviderSource{provider: ProviderOptimus},
			wantProvider: ProviderTendar,
			wantClient:   tendarClient,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(serviceDeps{
				ninPrembly: premblyClient,
				ninTendar:  tendarClient,
				source:     tc.source,
			})

			provider, client := service.ninProviderFor(context.Background())
			if provider != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", provider, tc.wantProvider)
			}
			if client != tc.wantClient {
				t.Fatalf("client = %v, want %v", client, tc.wantClient)
			}
		})
	}
}

func TestService_ValidateNIN_UnconfiguredProviderIsUnavailableNotInternal(t *testing.T) {
	// Preference says prembly but only the tendar client is wired — the user should
	// get a 503-mapped error, not a bare internal failure.
	service := newTestService(serviceDeps{
		ninTendar: &stubNINValidation{},
		source:    stubProviderSource{provider: ProviderPrembly},
	})

	_, err := service.ValidateNIN(context.Background(), "verification-1", "12345678901")
	if !errors.Is(err, appErr.ErrProviderServiceUnavailable) {
		t.Fatalf("expected %v, got %v", appErr.ErrProviderServiceUnavailable, err)
	}
}

func TestService_ValidateNIN_RejectsMalformedNINBeforeCallingProvider(t *testing.T) {
	client := &stubNINValidation{}
	service := newTestService(serviceDeps{
		ninTendar: client,
		source:    stubProviderSource{provider: ProviderTendar},
	})

	for _, number := range []string{"", "1234567890", "123456789012"} {
		_, err := service.ValidateNIN(context.Background(), "verification-1", number)
		if !errors.Is(err, appErr.ErrInvalidNIN) {
			t.Fatalf("nin %q: expected %v, got %v", number, appErr.ErrInvalidNIN, err)
		}
	}
	if client.called {
		t.Fatal("did not expect the provider to be called for a malformed NIN")
	}
}

func TestTranslateProviderError(t *testing.T) {
	notFound := appErr.ErrNINNotFound
	passthrough := errors.New("something else entirely")

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "nil stays nil",
			err:  nil,
			want: nil,
		},
		{
			name: "prembly unconfigured key is unavailable",
			err:  &appErr.PremblyError{Status: 500, Code: "CLIENT_ERROR", Message: "not configured"},
			want: appErr.ErrProviderServiceUnavailable,
		},
		{
			name: "tendar unconfigured key is unavailable",
			err:  &appErr.TendarError{Status: 500, Code: "CLIENT_ERROR", Message: "not configured"},
			want: appErr.ErrProviderServiceUnavailable,
		},
		{
			name: "retryable upstream failure is unavailable",
			err:  &appErr.TendarError{Status: 503, Code: "HTTP_ERROR", Retryable: true},
			want: appErr.ErrProviderServiceUnavailable,
		},
		{
			name: "rate limit is unavailable",
			err:  &appErr.PremblyError{Status: 429, Code: "HTTP_ERROR", Retryable: true},
			want: appErr.ErrProviderServiceUnavailable,
		},
		{
			name: "bad credentials are unavailable, never leaked",
			err:  &appErr.PremblyError{Status: 401, Code: "HTTP_ERROR", Message: "invalid api key"},
			want: appErr.ErrProviderServiceUnavailable,
		},
		{
			name: "upstream 404 means the identity was not found",
			err:  &appErr.PremblyError{Status: 404, Code: "HTTP_ERROR"},
			want: notFound,
		},
		{
			name: "tendar validation error means the identity was not found",
			err:  &appErr.TendarError{Status: 422, Code: "VALIDATION_ERROR", Message: "nin not found"},
			want: notFound,
		},
		{
			name: "unrecognised error passes through untouched",
			err:  passthrough,
			want: passthrough,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := translateProviderError(tc.err, notFound)
			if !errors.Is(got, tc.want) {
				t.Fatalf("translateProviderError() = %v, want %v", got, tc.want)
			}
		})
	}
}
