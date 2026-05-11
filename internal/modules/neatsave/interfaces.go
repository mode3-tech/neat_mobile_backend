package neatsave

import (
	"context"
	"neat_mobile_app_backend/internal/modules/device"
)

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}
