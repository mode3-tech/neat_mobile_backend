package middleware

import (
	"neat_mobile_app_backend/internal/modules/device"

	"golang.org/x/net/context"
)

type AccessTokenSigner interface {
	ValidAccessToken(token string) bool
	ExtractAccessTokenIdentifiers(token string) (sub, sid string, err error)
}

type SessionChecker interface {
	IsSessionActive(ctx context.Context, sid, mobileUserID, deviceID string) (bool, error)
}

type DeviceBindingChecker interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}
