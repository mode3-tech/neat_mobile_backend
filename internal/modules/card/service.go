package card

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"
	"neat_mobile_app_backend/internal/middleware"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	"neat_mobile_app_backend/providers/card"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo           *Repository
	deviceVerifier DeviceVerifier
	cardService    CardService
	auditLogger    auditlog.AuditLogger
}

func NewService(repo *Repository, deviceVerifier DeviceVerifier, cardService CardService, auditLogger auditlog.AuditLogger) *Service {
	return &Service{repo: repo, deviceVerifier: deviceVerifier, cardService: cardService, auditLogger: auditLogger}
}

func (s *Service) logCardAudit(ctx context.Context, mobileUserID, resourceID string, status auditlog.LogStatus, metadata map[string]interface{}) {
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   resourceID,
		ResourceType: auditlog.ResourceTypeCard,
		ActorType:    "user",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      mobileUserID,
		Action:       "CARD_REQUEST",
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

func (s *Service) RequestForCard(ctx context.Context, mobileUserID, deviceID string, payload RequestForCardRequest) error {
	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		if errors.Is(err, appErr.ErrUnrecognizedDevice) {
			return appErr.ErrUnrecognizedDeviceCardRequest
		}
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

	if err := s.cardService.RequestCard(ctx, &cSPayload); err != nil {
		s.logCardAudit(ctx, mobileUserID, referenceID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "provider card request failed",
		})
		return err
	}

	s.logCardAudit(ctx, mobileUserID, referenceID, auditlog.StatusSuccess, map[string]interface{}{
		"delivery_fee":     payload.DeliveryFee,
		"is_branch_pickup": true,
	})

	return nil
}
