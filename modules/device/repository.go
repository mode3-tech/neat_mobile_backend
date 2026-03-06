package device

import (
	"context"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Save(ctx context.Context, device *UserDevice) error {
	return r.db.WithContext(ctx).Create(device).Error
}

func (r *DeviceRepository) FindDevice(ctx context.Context, userID, deviceID string) (*UserDevice, error) {
	var device UserDevice
	if err := r.db.WithContext(ctx).Table("user_devices").Select("*").Where("user_id = ? AND device_id = ?", userID, deviceID).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) CreateChallenge(ctx context.Context, ch *DeviceChallenge) error {
	now := time.Now().UTC()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Ensure at most one active challenge exists for a user/device pair.
		var locked UserDevice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND device_id = ?", ch.UserID, ch.DeviceID).
			First(&locked).Error; err != nil {
			return err
		}

		if err := tx.Model(&DeviceChallenge{}).
			Where("user_id = ? AND device_id = ? AND used_at IS NULL", ch.UserID, ch.DeviceID).
			Updates(map[string]any{
				"used_at":    now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		return tx.Create(ch).Error
	})
}

func (r *DeviceRepository) GetChallengeByHash(ctx context.Context, challengeHash string) (*DeviceChallenge, error) {
	var challenge DeviceChallenge
	if err := r.db.WithContext(ctx).Table("device_challenges").Select("*").Where("challenge_hash = ?", challengeHash).First(&challenge).Error; err != nil {
		return nil, err
	}

	return &challenge, nil
}

func (r *DeviceRepository) MarkChallengeUsed(ctx context.Context, id string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&DeviceChallenge{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]any{
			"used_at":    now,
			"updated_at": now,
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

func (r *DeviceRepository) CreatePendingSession(ctx context.Context, session *models.PendingDeviceSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *DeviceRepository) GetPendingSessionByHash(ctx context.Context, tokenHash string) (*models.PendingDeviceSession, error) {
	var session models.PendingDeviceSession
	if err := r.db.WithContext(ctx).
		Table("pending_device_sessions").
		Select("*").
		Where("session_token_hash = ?", tokenHash).
		First(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *DeviceRepository) MarkPendingSessionUsed(ctx context.Context, id string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.PendingDeviceSession{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]any{
			"used_at":    now,
			"updated_at": now,
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

func (r *DeviceRepository) UpsertDevicePublicKey(ctx context.Context, device *UserDevice) error {
	if device == nil {
		return gorm.ErrInvalidData
	}

	now := time.Now().UTC()
	if device.LastUsedAt.IsZero() {
		device.LastUsedAt = now
	}

	// Security invariant:
	// - Trust/active state must never be caller-controlled in this method.
	// - New rows are created as untrusted by default.
	// - Existing rows keep their current trust/active state.
	safeInsert := *device
	safeInsert.IsTrusted = false
	safeInsert.IsActive = true

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "device_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"public_key":   device.PublicKey,
				"device_name":  device.DeviceName,
				"device_model": device.DeviceModel,
				"os":           device.OS,
				"os_version":   device.OSVersion,
				"app_version":  device.AppVersion,
				"last_used_at": device.LastUsedAt,
			}),
		}).
		Create(&safeInsert).Error
}
