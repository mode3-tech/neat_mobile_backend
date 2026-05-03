package device

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo Repository
}

const challengeTTL = 5 * time.Minute

var ErrDeviceNotEligible = errors.New("device not eligible for challenge")

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) BindDevice(ctx context.Context, userID string, req *DeviceBindingRequest) error {
	device := &UserDevice{
		ID:          req.DeviceID,
		UserID:      userID,
		DeviceID:    req.DeviceID,
		PublicKey:   req.PublicKey,
		DeviceName:  req.DeviceName,
		DeviceModel: req.DeviceModel,
		OS:          req.OS,
		OSVersion:   req.OSVersion,
		IP:          req.IP,
		AppVersion:  req.AppVersion,
		IsTrusted:   true,
		IsActive:    true,
	}
	return s.repo.Save(ctx, device)
}

func (s *Service) CreateChallenge(ctx context.Context, userID, deviceID string, ttl time.Duration) (string, error) {
	device, err := s.repo.FindDevice(ctx, userID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", appErr.ErrUnauthorized
		}
		return "", err
	}

	if !device.IsActive || !device.IsTrusted {
		return "", appErr.ErrUnauthorized
	}

	rawChallenge, err := randomToken(32)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(rawChallenge))
	challengeHash := hex.EncodeToString(sum[:])

	if ttl == 0 {
		ttl = challengeTTL
	}

	now := time.Now().UTC()
	row := &DeviceChallenge{
		ID:            uuid.NewString(),
		UserID:        userID,
		DeviceID:      deviceID,
		ChallengeHash: challengeHash,
		ExpiresAt:     now.Add(ttl),
		CreatedAt:     now,
		UpdatedAt:     now,
		UsedAt:        nil,
	}

	if err := s.repo.CreateChallenge(ctx, row); err != nil {
		return "", err
	}

	return rawChallenge, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*UserDevice, error) {
	userDevice, err := s.repo.FindDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("device not found")
		}
		return nil, err
	}

	if !userDevice.IsActive || !userDevice.IsTrusted {
		return nil, errors.New("device not allowed")
	}
	return userDevice, nil
}
