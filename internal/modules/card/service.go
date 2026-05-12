package card

import (
	"context"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/providers/card"

	"github.com/google/uuid"
)

type Service struct {
	repo           *Repository
	deviceVerifier DeviceVerifier
	cardService    CardService
}

func NewService(repo *Repository, deviceVerifier DeviceVerifier, cardService CardService) *Service {
	return &Service{repo: repo, deviceVerifier: deviceVerifier, cardService: cardService}
}

func (s *Service) RequestForCard(ctx context.Context, mobileUserID, deviceID string, payload RequestForCardRequest) error {
	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	wallet, err := s.repo.FindWalletWithMobileUserID(ctx, mobileUserID)
	if err != nil {
		return appErr.ErrRequestingForCard
	}

	if payload.DeliveryFee < 0 {
		return appErr.ErrInvalidTransferAmount
	}

	if payload.DeliveryFee > 0 && payload.DeliveryFee < wallet.AvailableBalance {
		return appErr.ErrInsufficientBalance
	}

	referenceID := uuid.NewString()

	cSPayload := card.OptimusCardRequest{
		ReferenceID:          referenceID,
		IsBranchPickup:       true,
		IsHomeDelivery:       false,
		AccountToLink:        wallet.AccountNumber,
		AccountToDebit:       wallet.AccountNumber,
		Reason:               "NEW_CARD",
		CardType:             "Verve",
		HouseNumber:          payload.HouseNumber,
		StreetName:           payload.StreetName,
		State:                payload.State,
		LGA:                  payload.LGA,
		DeliveryFee:          payload.DeliveryFee,
		City:                 payload.City,
		BranchPickupLocation: payload.BranchPickupLocation,
	}

	return s.cardService.RequestCard(ctx, &cSPayload)
}
