package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"neat_mobile_app_backend/modules/device"
	"strings"
	"time"

	"gorm.io/gorm"
)

const registrationClaimTokenTTL = 24 * time.Hour

func newRegistrationClaimToken(now time.Time) (string, string, time.Time, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	token, err := randomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return token, hashRegistrationClaimToken(token), now.Add(registrationClaimTokenTTL), nil
}

func hashRegistrationClaimToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func registrationJobCanClaimAt(job *RegistrationJob, now time.Time) bool {
	if job == nil || job.Status != RegistrationJobStatusCompleted {
		return false
	}
	if job.SessionClaimedAt != nil {
		return false
	}
	if job.SessionClaimTokenHash == nil || strings.TrimSpace(*job.SessionClaimTokenHash) == "" {
		return false
	}
	if job.SessionClaimExpiresAt != nil && now.After(*job.SessionClaimExpiresAt) {
		return false
	}

	return true
}

func (s *Service) ClaimRegistrationSession(ctx context.Context, jobID, claimToken, deviceID, ip string) (*VerifiedDeviceResponse, error) {
	if s.tx == nil {
		return nil, errors.New("transaction manager not configured")
	}

	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, errors.New("job id is required")
	}

	claimToken = strings.TrimSpace(claimToken)
	if claimToken == "" {
		return nil, errors.New("claim token is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	var resp *VerifiedDeviceResponse

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		deviceRepo := device.NewRepository(txDB)

		job, err := authRepo.GetRegistrationJobByIDForUpdate(ctx, jobID)
		if err != nil {
			return err
		}

		switch job.Status {
		case RegistrationJobStatusCompleted:
		case RegistrationJobStatusFailed:
			return errors.New("registration failed")
		default:
			return errors.New("registration is not completed")
		}

		now := time.Now().UTC()
		if !registrationJobCanClaimAt(job, now) {
			switch {
			case job.SessionClaimedAt != nil:
				return errors.New("registration session already claimed")
			case job.SessionClaimExpiresAt != nil && now.After(*job.SessionClaimExpiresAt):
				return errors.New("registration session expired")
			default:
				return errors.New("registration session unavailable")
			}
		}

		if job.SessionClaimTokenHash == nil || *job.SessionClaimTokenHash != hashRegistrationClaimToken(claimToken) {
			return errors.New("invalid registration claim token")
		}

		snapshot, err := decodeRegistrationSnapshot(job.SnapshotJSON)
		if err != nil {
			return err
		}

		if strings.TrimSpace(snapshot.Device.DeviceID) != deviceID {
			return errors.New("device not allowed")
		}

		deviceRecord, err := deviceRepo.FindDevice(ctx, job.MobileUserID, deviceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("device not found")
			}
			return err
		}
		if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
			return errors.New("device not allowed")
		}

		if err := authRepo.MarkRegistrationJobClaimed(ctx, job.ID, now); err != nil {
			return err
		}

		resp, err = s.issueSessionTokensWithRepo(ctx, authRepo, job.MobileUserID, deviceID, strings.TrimSpace(ip))
		if err != nil {
			return err
		}

		user, err := authRepo.GetUserByID(ctx, job.MobileUserID)
		if err != nil {
			return err
		}
		resp.IsBiometricsEnabled = user.IsBiometricsEnabled

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}
