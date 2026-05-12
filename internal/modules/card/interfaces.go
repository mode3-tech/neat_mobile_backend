package card

import (
	"context"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/providers/card"
)

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}

type CardService interface {
	RequestCard(ctx context.Context, requestInfo *card.OptimusCardRequest) error
}
