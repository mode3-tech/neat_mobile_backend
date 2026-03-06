package device

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeviceService struct {
	repo DeviceRepository
}

const challengeTTL = 60 * time.Second

var ErrDeviceNotEligible = errors.New("device not eligible for challenge")

func NewDeviceService(repo DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo}
}

func (s *DeviceService) BindDevice(ctx context.Context, userID string, req *DeviceBindingRequest) error {
	device := &UserDevice{
		ID:          req.DeviceID,
		UserID:      userID,
		DeviceID:    req.DeviceID,
		PublicKey:   req.PublicKey,
		DeviceName:  req.DeviceName,
		DeviceModel: req.DeviceModel,
		OS:          req.OS,
		OSVersion:   req.OSVersion,
		AppVersion:  req.AppVersion,
		IsTrusted:   true,
		IsActive:    true,
	}
	return s.repo.Save(ctx, device)
}

func (s *DeviceService) CreateChallenge(ctx context.Context, userID, deviceID string) (string, error) {
	device, err := s.repo.FindDevice(ctx, userID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrDeviceNotEligible
		}
		return "", err
	}

	if !device.IsActive || !device.IsTrusted {
		return "", ErrDeviceNotEligible
	}

	rawChallenge, err := randomToken(32)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(rawChallenge))
	challengeHash := hex.EncodeToString(sum[:])

	now := time.Now().UTC()
	row := &DeviceChallenge{
		ID:            uuid.NewString(),
		UserID:        userID,
		DeviceID:      deviceID,
		ChallengeHash: challengeHash,
		ExpiresAt:     now.Add(challengeTTL),
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
