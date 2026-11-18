// Package wema adapts the concrete *baas.Alat client to the auth package's
// WalletService interface and registerv2's AlatValidator interface.
package wema

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth"
	registerv2 "neat_mobile_app_backend/internal/modules/auth/registerv2"
	"neat_mobile_app_backend/providers/baas"

	"github.com/google/uuid"
)

// Adapter wraps *baas.Alat so it satisfies auth.WalletService and registerv2.AlatValidator.
type Adapter struct {
	alat *baas.Alat
	// pollInterval spaces GenerateWallet's account polls (shortened in tests).
	pollInterval time.Duration
}

// New returns an Adapter that implements auth.WalletService and registerv2.AlatValidator.
func New(alat *baas.Alat) *Adapter {
	return &Adapter{alat: alat, pollInterval: time.Second}
}

// Client returns the underlying *baas.Alat client.
func (a *Adapter) Client() *baas.Alat {
	return a.alat
}

// LookupWalletByCustomerID finds an existing ALAT account by phone number.
// found=false with a nil error means ALAT answered that there is no account.
func (a *Adapter) LookupWalletByCustomerID(ctx context.Context, phoneNumber string) (*auth.WalletResponse, bool, error) {
	details, err := a.GetPartnershipAccountDetails(ctx, phoneNumber)
	if err != nil {
		if isAlatAnswer(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if details == nil || strings.TrimSpace(details.AccountNumber) == "" {
		return nil, false, nil
	}
	return &auth.WalletResponse{
		Customer: &auth.WalletCustomer{ID: phoneNumber, Currency: "NGN"},
		Wallet: &auth.WalletInfo{
			AccountNumber: details.AccountNumber,
			AccountName:   strings.TrimSpace(details.FirstName + " " + details.LastName),
			BankCode:      baas.WemaBankCode,
			BankName:      baas.WemaBankName,
			WalletId:      details.AccountNumber,
		},
	}, true, nil
}

// GenerateWallet creates a tier 1 (BVN) account, or returns the existing one,
// and lifts PND so it can be debited. Used by v1 registration jobs when
// WALLET_PROVIDER is wema.
func (a *Adapter) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	if walletInfo == nil {
		return nil, errors.New("wallet payload is required")
	}

	// A retried job may have created the account already. A lookup that got
	// no answer says nothing about whether it exists, so it must not lead on
	// to creating a second one.
	existing, found, err := a.LookupWalletByCustomerID(ctx, walletInfo.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("alat account lookup failed: %w", err)
	}
	if found {
		// An earlier attempt may have created the account and then failed to lift PND.
		if err := a.liftPnd(ctx, existing.Wallet.AccountNumber); err != nil {
			return nil, err
		}
		return existing, nil
	}

	resp, err := a.alat.CreateTier1AlatAccountWithBVN(ctx, baas.AlatTier1BVNRequest{
		PhoneNumber:   walletInfo.PhoneNumber,
		Email:         walletInfo.Email,
		BVN:           walletInfo.BVN,
		CorrelationID: uuid.NewString(),
	})
	if err == nil {
		err = envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
	}
	if err != nil {
		return nil, fmt.Errorf("alat wallet creation failed: %w", err)
	}

	// Creation is queued; poll briefly for the account.
	const maxPollAttempts = 4
	for attempt := 1; attempt <= maxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(a.pollInterval):
		}
		wallet, found, lookupErr := a.LookupWalletByCustomerID(ctx, walletInfo.PhoneNumber)
		if lookupErr == nil && found {
			// On failure the job retries, finds the account above and lifts PND there.
			if err := a.liftPnd(ctx, wallet.Wallet.AccountNumber); err != nil {
				return nil, err
			}
			return wallet, nil
		}
	}

	log.Printf("[ALAT] level=WARN op=GenerateWallet account not visible after %d polls phone=%s - a retry should find it",
		maxPollAttempts, maskTail(walletInfo.PhoneNumber))
	return nil, errors.New("alat account generation pending; please check status shortly")
}

func (a *Adapter) liftPnd(ctx context.Context, accountNumber string) error {
	if err := a.PartnerDebitRestrictionManagement(ctx, string(baas.AlatPndTypeLift), accountNumber); err != nil {
		log.Printf("[ALAT] level=ERROR op=liftPnd account=%s: %v", maskTail(accountNumber), err)
		return fmt.Errorf("alat lift pnd failed for account %s: %w", accountNumber, err)
	}
	return nil
}

// PostPartnershipAccountCreationWithBvn is Wallet Creation (BVN) Step 1. It returns the trackingId for Step 2.
func (a *Adapter) PostPartnershipAccountCreationWithBvn(ctx context.Context, phone, email, bvn string) (string, error) {
	resp, err := a.alat.PostPartnershipAccountCreationWithBvn(ctx, baas.AlatPartnershipRequestWithBvn{
		PhoneNumber: phone,
		Email:       email,
		BVN:         bvn,
	})
	if err != nil {
		return "", err
	}
	if err := envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors); err != nil {
		return "", err
	}
	if resp.Data == nil {
		return "", nil
	}
	return resp.Data.TrackingID, nil
}

// ValidateBVNandEnqueueAccountCreation is Wallet Creation (BVN) Step 2: it confirms the customer's OTP.
func (a *Adapter) ValidateBVNandEnqueueAccountCreation(ctx context.Context, phone, otp, trackingID string) error {
	resp, err := a.alat.ValidateBVNandEnqueueAccountCreation(ctx, baas.AlatValidateBvnRequest{
		PhoneNumber: phone,
		OTP:         otp,
		TrackingID:  trackingID,
	})
	if err != nil {
		return err
	}
	return envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
}

// ResendOtp asks ALAT to resend the Wallet Creation (BVN) OTP.
func (a *Adapter) ResendOtp(ctx context.Context, trackingID string) error {
	resp, err := a.alat.ResendOtp(ctx, baas.AlatResendOtpRequest{TrackingID: trackingID})
	if err != nil {
		return err
	}
	return envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
}

// GetPartnershipAccountDetails is face biometric Step 2: the account opened
// for phoneNumber. A nil result with a nil error, or an *appErr.AlatError ALAT
// sent, means no account; any other error means ALAT couldn't be asked.
func (a *Adapter) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*registerv2.AlatAccountDetails, error) {
	resp, err := a.alat.GetAccountDetailsWithPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}
	if err := envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors); err != nil {
		return nil, err
	}
	if resp.Data == nil || strings.TrimSpace(resp.Data.AccountNumber) == "" {
		return nil, nil
	}
	d := resp.Data
	return &registerv2.AlatAccountDetails{
		AccountNumber: strings.TrimSpace(d.AccountNumber),
		FirstName:     d.FirstName,
		LastName:      d.LastName,
		Email:         d.Email,
		PhoneNumber:   d.PhoneNumber,
	}, nil
}

// PartnerDebitRestrictionManagement is Wallet Creation (BVN) Step 4: lift ("LiftPnd") or place ("PlacePnd") PND.
func (a *Adapter) PartnerDebitRestrictionManagement(ctx context.Context, pndType, accountNumber string) error {
	resp, err := a.alat.PartnerDebitRestrictionManagement(ctx, baas.AlatPndRequest{
		PndType:       baas.AlatPndType(pndType),
		AccountNumber: accountNumber,
	})
	if err != nil {
		return err
	}
	return envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
}

// PostTier2PartnershipAccountWithFace is face biometric Step 1c: it queues a tier 2 account and returns the trackingId.
func (a *Adapter) PostTier2PartnershipAccountWithFace(ctx context.Context, req registerv2.AlatTier2FaceRequest) (string, error) {
	var address *baas.ResidentialAddress
	if r := req.ResidentialAddress; r != nil {
		address = &baas.ResidentialAddress{
			BuildingNumber: r.BuildingNumber,
			Street:         r.Street,
			City:           r.City,
			State:          r.State,
			LGA:            r.Lga,
			Landmark:       r.Landmark,
			Country:        r.Country,
			FullAddress:    r.FullAddress,
		}
	}
	resp, err := a.alat.CreateTier2AlatAccount(ctx, baas.AlatAccountCreationRequest{
		BVN:                req.BVN,
		NIN:                req.NIN,
		PhoneNumber:        req.PhoneNumber,
		EmailAddress:       req.EmailAddress,
		ResidentialAddress: address,
		LiveImageOfFace:    req.LiveImageOfFace,
		CorrelationID:      req.CorrelationID,
	})
	if err != nil {
		return "", err
	}
	if err := envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors); err != nil {
		return "", err
	}
	if resp.Data == nil {
		return "", nil
	}
	return resp.Data.TrackingID, nil
}

// GetAddressDropDownList is face biometric Step 0: country, state, LGA, LCDA, city and housing type options.
func (a *Adapter) GetAddressDropDownList(ctx context.Context) (any, error) {
	resp, err := a.alat.GetAddressDropdownList(ctx)
	if err != nil {
		return nil, err
	}
	if err := envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors); err != nil {
		return nil, err
	}
	return resp, nil
}

// envelopeError turns a {message, status, code, statusCode, errors} envelope
// with status=false into the error ALAT meant.
func envelopeError(status bool, message string, code baas.AlatResponseCode, statusCode baas.AlatHTTPStatusCode, errs []string) error {
	if status {
		return nil
	}
	if strings.TrimSpace(message) == "" {
		message = "alat request was not successful"
	}
	return &appErr.AlatError{StatusCode: string(statusCode), Code: string(code), Message: message, Errors: errs}
}

// resultError turns a {result, errorMessage, errorMessages, hasError} envelope
// with hasError=true into an error.
func resultError(hasError bool, message string, messages []string) error {
	if !hasError {
		return nil
	}
	if strings.TrimSpace(message) == "" {
		message = strings.Join(messages, "; ")
	}
	if strings.TrimSpace(message) == "" {
		message = "alat request was not successful"
	}
	return &appErr.AlatError{Message: message, Errors: messages}
}

// isAlatAnswer reports whether err is a reply ALAT actually sent (as opposed
// to a timeout, transport failure or outage).
func isAlatAnswer(err error) bool {
	var alatErr *appErr.AlatError
	return errors.As(err, &alatErr) && !alatErr.NoResponse
}

func maskTail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// Compile-time interface checks.
var (
	_ auth.WalletService       = (*Adapter)(nil)
	_ registerv2.AlatValidator = (*Adapter)(nil)
)
