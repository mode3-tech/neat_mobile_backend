// Package wema adapts the concrete *baas.Alat client to the auth package's
// WalletService interface and registerv2's AlatValidator interface.
package wema

import (
	"context"
	"neat_mobile_app_backend/internal/modules/auth"
	registerv2 "neat_mobile_app_backend/internal/modules/auth/registerv2"
	"neat_mobile_app_backend/providers/baas"
)

// Adapter wraps *baas.Alat so it satisfies auth.WalletService and registerv2.AlatValidator.
type Adapter struct {
	alat *baas.Alat
}

// New returns an Adapter that implements auth.WalletService and registerv2.AlatValidator.
func New(alat *baas.Alat) *Adapter {
	return &Adapter{alat: alat}
}

// GenerateWallet initiates or creates an account on ALAT.
func (a *Adapter) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	return a.alat.GenerateWallet(ctx, walletInfo)
}

// LookupWalletByCustomerID queries ALAT for account details.
func (a *Adapter) LookupWalletByCustomerID(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
	return a.alat.LookupWalletByCustomerID(ctx, customerID)
}

// Client returns the underlying *baas.Alat client.
func (a *Adapter) Client() *baas.Alat {
	return a.alat
}

// PostPartnershipAccountCreationWithBvn calls Step 1 on ALAT and returns trackingID.
func (a *Adapter) PostPartnershipAccountCreationWithBvn(ctx context.Context, phone, email, bvn string) (string, error) {
	resp, err := a.alat.PostPartnershipAccountCreationWithBvn(ctx, baas.AlatPartnershipRequestWithBvn{
		PhoneNumber: phone,
		Email:       email,
		BVN:         bvn,
	})
	if err != nil {
		return "", err
	}
	trackingID := ""
	if resp != nil && resp.Data != nil {
		trackingID = resp.Data.TrackingID
	}
	return trackingID, nil
}

// ValidateBVNandEnqueueAccountCreation calls Step 2 on ALAT to confirm customer OTP.
func (a *Adapter) ValidateBVNandEnqueueAccountCreation(ctx context.Context, phone, otp, trackingID string) error {
	_, err := a.alat.ValidateBVNandEnqueueAccountCreation(ctx, baas.AlatValidateBvnRequest{
		PhoneNumber: phone,
		OTP:         otp,
		TrackingID:  trackingID,
	})
	return err
}

// GetPartnershipAccountDetails calls Step 3 on ALAT to retrieve account details.
func (a *Adapter) GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*registerv2.AlatAccountDetails, error) {
	resp, err := a.alat.GetPartnershipAccountDetails(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Data == nil {
		return nil, nil
	}
	return &registerv2.AlatAccountDetails{
		AccountNumber: resp.Data.AccountNumber,
		FirstName:     resp.Data.FirstName,
		LastName:      resp.Data.LastName,
		Email:         resp.Data.Email,
		PhoneNumber:   resp.Data.PhoneNumber,
	}, nil
}

// PartnerDebitRestrictionManagement calls Step 4 on ALAT to lift or place PND.
func (a *Adapter) PartnerDebitRestrictionManagement(ctx context.Context, pndType, accountNumber string) error {
	_, err := a.alat.PartnerDebitRestrictionManagement(ctx, baas.AlatPndRequest{
		PndType:       pndType,
		AccountNumber: accountNumber,
	})
	return err
}

// ResendOtp requests ALAT to resend verification OTP.
func (a *Adapter) ResendOtp(ctx context.Context, trackingID string) error {
	_, err := a.alat.ResendOtp(ctx, baas.AlatResendOtpRequest{
		TrackingID: trackingID,
	})
	return err
}
