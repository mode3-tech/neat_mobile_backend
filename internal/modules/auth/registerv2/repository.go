package registerv2

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"neat_mobile_app_backend/internal/crypto"
	"neat_mobile_app_backend/models"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db     *gorm.DB
	cipher *crypto.FieldCipher
}

func NewRepository(db *gorm.DB, cipher *crypto.FieldCipher) *Repository {
	return &Repository{
		db:     db,
		cipher: cipher,
	}
}

// decryptVerifiedID reverses AddVerification's encryption of VerifiedID.
// Safe on rows written before encryption was introduced - Decrypt passes
// legacy plaintext through unchanged.
func (r *Repository) decryptVerifiedID(rec *models.VerificationRecord) error {
	if rec == nil || rec.VerifiedID == nil {
		return nil
	}
	plain, err := r.cipher.Decrypt(*rec.VerifiedID)
	if err != nil {
		return err
	}
	rec.VerifiedID = &plain
	return nil
}

// GetVerificationByID looks up a verification record by its ID. It takes no
// row lock: callers aren't in a transaction, so FOR UPDATE would be
// released immediately. Double use of a record is prevented instead by
// MarkVerificationUsed's status-guarded update inside the registration
// transaction, and by the per-person registration lock.
func (r *Repository) GetVerificationByID(ctx context.Context, id string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Limit(1).
		Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	if err := r.decryptVerifiedID(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetLatestByTypeAndHash returns the most recent verification record for an
// identity number (BVN/NIN) by its subject hash, regardless of status - there's
// no client-facing ID for these, the client resends the raw value and we
// re-derive the hash to look it up. Register uses the status to tell apart
// "never attempted" (nil), "Optimus rejected it" (failed - fallback eligible),
// "OTP not yet confirmed" (pending - not fallback eligible), and "verified".
func (r *Repository) GetLatestByTypeAndHash(ctx context.Context, verificationType, subjectHash string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Where("type = ? AND subject_hash = ?", verificationType, subjectHash).
		Order("created_at DESC").
		Limit(1).
		Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	if err := r.decryptVerifiedID(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// MarkVerificationVerifiedByProviderReference flips a pending BVN/NIN
// verification record to verified once its Optimus OTP challenge is
// confirmed - looked up by the provider's reference id, since that's the
// only identifier VerifyOTP has to go on.
func (r *Repository) MarkVerificationVerifiedByProviderReference(ctx context.Context, providerReferenceID string, verifiedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("provider_verification_id = ? AND status = ?", providerReferenceID, models.VerificationStatusPending).
		Updates(map[string]any{"status": models.VerificationStatusVerified, "verified_at": verifiedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record not found or already verified")
	}
	return nil
}

// MarkVerificationVerified sets a verification record's status to verified by its record ID.
// Validates that the record exists, has type BVN, is currently pending, and is not expired.
func (r *Repository) MarkVerificationVerified(ctx context.Context, id string, verifiedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("id = ? AND type = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", id, models.VerificationTypeBVN, models.VerificationStatusPending, verifiedAt).
		Updates(map[string]any{"status": models.VerificationStatusVerified, "verified_at": verifiedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record not eligible for verification (not pending, wrong type, or expired)")
	}
	return nil
}

// GetVerificationByProviderReference finds the latest verification record by its provider reference ID.
func (r *Repository) GetVerificationByProviderReference(ctx context.Context, providerReferenceID string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Where("provider_verification_id = ?", strings.TrimSpace(providerReferenceID)).
		Order("created_at DESC").
		Limit(1).
		Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	if err := r.decryptVerifiedID(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateProviderVerificationReference repoints a pending verification record
// from an old Optimus OTP reference id to the new one issued by a resend -
// without this, VerifyOTP's lookup by reference id would miss the record,
// since the client is required to confirm against the new id, not the old one.
func (r *Repository) UpdateProviderVerificationReference(ctx context.Context, oldReferenceID, newReferenceID string) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("provider_verification_id = ? AND status = ?", oldReferenceID, models.VerificationStatusPending).
		Update("provider_verification_id", newReferenceID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record not found or already verified")
	}
	return nil
}

// MarkVerificationUsed mirrors verification.VerificationRepo.MarkVerificationUsed.
func (r *Repository) MarkVerificationUsed(ctx context.Context, id string, usedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("id = ? AND status = ?", id, models.VerificationStatusVerified).
		Updates(map[string]any{"status": models.VerificationStatusUsed, "used_at": usedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record already used or not found")
	}
	return nil
}

// GetProviderPreference returns the current wallet provider preference row.
func (r *Repository) GetProviderPreference(ctx context.Context) (*ProviderPreference, error) {
	var pref ProviderPreference
	if err := r.db.WithContext(ctx).First(&pref, "id = ?", 1).Error; err != nil {
		return nil, err
	}
	return &pref, nil
}

func (r *Repository) AddVerification(ctx context.Context, record *models.VerificationRecord) error {
	if record.VerifiedID == nil {
		return r.db.WithContext(ctx).Create(record).Error
	}
	// Encrypt into a copy so the caller's struct keeps its plaintext
	// VerifiedID - the caller (registerv2/service.go) doesn't reuse it after
	// this call today, but matching the same defensive pattern used in
	// auth.Repository's CreateUser/CreateBVNRecord keeps this safe regardless.
	toInsert := *record
	encrypted, err := r.cipher.Encrypt(*record.VerifiedID)
	if err != nil {
		return err
	}
	toInsert.VerifiedID = &encrypted
	return r.db.WithContext(ctx).Create(&toInsert).Error
}

// UserExistsByIdentity reports whether a user already exists with this BVN or
// NIN. wallet_users.bvn_hash/nin_hash are unique, so without this check a
// duplicate would only surface at CreateUser - after the provider account had
// already been created. Uses the same crypto.Hash that auth.Repository.CreateUser
// writes. An empty value is skipped rather than matched against hash("").
func (r *Repository) UserExistsByIdentity(ctx context.Context, bvn, nin string) (bool, error) {
	bvn = strings.TrimSpace(bvn)
	nin = strings.TrimSpace(nin)

	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if bvn != "" {
		conditions = append(conditions, "bvn_hash = ?")
		args = append(args, crypto.Hash(bvn))
	}
	if nin != "" {
		conditions = append(conditions, "nin_hash = ?")
		args = append(args, crypto.Hash(nin))
	}
	if len(conditions) == 0 {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where(strings.Join(conditions, " OR "), args...).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetFaceCheckRecord(ctx context.Context, id string) (*models.FaceCheckRecord, error) {
	var record models.FaceCheckRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, err
	}
	if err := r.decryptFaceImage(&record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Repository) GetFaceCheckRecordByVerificationID(ctx context.Context, verificationRecordID string) (*models.FaceCheckRecord, error) {
	var record models.FaceCheckRecord
	if err := r.db.WithContext(ctx).
		Where("verification_record_id = ?", verificationRecordID).
		Order("created_at DESC").
		First(&record).Error; err != nil {
		return nil, err
	}
	if err := r.decryptFaceImage(&record); err != nil {
		return nil, err
	}
	return &record, nil
}

// decryptFaceImage reverses auth.Repository.CreateFaceCheckRecord's
// encryption of the selfie. Legacy plaintext rows pass through unchanged.
func (r *Repository) decryptFaceImage(record *models.FaceCheckRecord) error {
	if record == nil || record.FaceImageProvided == nil || *record.FaceImageProvided == "" || r.cipher == nil {
		return nil
	}
	plain, err := r.cipher.Decrypt(*record.FaceImageProvided)
	if err != nil {
		return err
	}
	record.FaceImageProvided = &plain
	return nil
}

// --- Pending Wema registrations -------------------------------------------

func (r *Repository) CreatePendingWemaRegistration(ctx context.Context, p *PendingWemaRegistration) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *Repository) GetPendingWemaRegistration(ctx context.Context, id string) (*PendingWemaRegistration, error) {
	var p PendingWemaRegistration
	res := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(id)).Limit(1).Find(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &p, nil
}

// GetActivePendingWemaRegistrationByPhone returns the pending/processing row
// for phone, if any (at most one, by idx_pending_wema_active_phone).
func (r *Repository) GetActivePendingWemaRegistrationByPhone(ctx context.Context, phone string) (*PendingWemaRegistration, error) {
	var p PendingWemaRegistration
	res := r.db.WithContext(ctx).
		Where("phone = ? AND status IN ?", phone, []string{PendingWemaStatusPending, PendingWemaStatusProcessing}).
		Limit(1).Find(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &p, nil
}

// RefreshPendingWemaRegistration replaces a pending row's customer-supplied
// data with a newer /register call's (the latest attempt wins) and rotates
// its claim token. Only a row that is still pending is updated.
func (r *Repository) RefreshPendingWemaRegistration(ctx context.Context, p *PendingWemaRegistration) (bool, error) {
	res := r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status = ?", p.ID, PendingWemaStatusPending).
		Updates(map[string]any{
			"email": p.Email, "first_name": p.FirstName, "last_name": p.LastName, "dob": p.DOB,
			"address": p.Address, "password_hash": p.PasswordHash, "pin_hash": p.PinHash,
			"biometrics": p.Biometrics, "referral_code": p.ReferralCode,
			"device_id": p.DeviceID, "device_public_key": p.DevicePublicKey, "device_name": p.DeviceName,
			"device_model": p.DeviceModel, "device_os": p.DeviceOS, "device_os_version": p.DeviceOSVersion,
			"device_app_version": p.DeviceAppVersion, "ip": p.IP,
			"phone_verification_id": p.PhoneVerification, "email_verification_id": p.EmailVerification,
			"bvn_verification_id": p.BVNVerification, "claim_token_hash": p.ClaimTokenHash,
		})
	return res.RowsAffected == 1, res.Error
}

// StartPendingWemaAttempt atomically moves a pending row to processing, so
// the webhook, sweep, status poll and a retried /register never complete the
// same registration twice. false means another worker has it (or it's done).
func (r *Repository) StartPendingWemaAttempt(ctx context.Context, id string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status = ?", id, PendingWemaStatusPending).
		Updates(map[string]any{
			"status":          PendingWemaStatusProcessing,
			"attempts":        gorm.Expr("attempts + 1"),
			"last_attempt_at": now,
		})
	return res.RowsAffected == 1, res.Error
}

// ReleasePendingWemaAttempt hands a processing row back to pending after an
// attempt that should be retried later.
func (r *Repository) ReleasePendingWemaAttempt(ctx context.Context, id, reason string) error {
	return r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status = ?", id, PendingWemaStatusProcessing).
		Updates(map[string]any{"status": PendingWemaStatusPending, "failure_reason": reason}).Error
}

func (r *Repository) FailPendingWemaRegistration(ctx context.Context, id, reason string) error {
	return r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status IN ?", id, []string{PendingWemaStatusPending, PendingWemaStatusProcessing}).
		Updates(map[string]any{"status": PendingWemaStatusFailed, "failure_reason": reason}).Error
}

// CompletePendingWemaRegistration marks a processing row completed. Called
// inside the transaction that saves the user, so both commit together.
func (r *Repository) CompletePendingWemaRegistration(ctx context.Context, id, accountNumber string, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status = ?", id, PendingWemaStatusProcessing).
		Updates(map[string]any{
			"status": PendingWemaStatusCompleted, "account_number": accountNumber,
			"completed_at": now, "failure_reason": nil,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("pending wema registration is no longer processing")
	}
	return nil
}

// ClaimPendingWemaRegistration marks a completed row's session as handed out.
// false means it was already claimed.
func (r *Repository) ClaimPendingWemaRegistration(ctx context.Context, id string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status = ? AND claimed_at IS NULL", id, PendingWemaStatusCompleted).
		Update("claimed_at", now)
	return res.RowsAffected == 1, res.Error
}

// ResetStuckPendingWemaAttempts returns rows left processing by a worker that
// died mid-attempt (older than before) to pending.
func (r *Repository) ResetStuckPendingWemaAttempts(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("status = ? AND last_attempt_at < ?", PendingWemaStatusProcessing, before).
		Update("status", PendingWemaStatusPending)
	return res.RowsAffected, res.Error
}

// ExpirePendingWemaRegistrations expires pending rows past their deadline and
// returns them, so the caller can flag ALAT accounts that may be orphaned.
func (r *Repository) ExpirePendingWemaRegistrations(ctx context.Context, now time.Time) ([]PendingWemaRegistration, error) {
	var rows []PendingWemaRegistration
	if err := r.db.WithContext(ctx).
		Clauses(clause.Returning{}).
		Model(&rows).
		Where("status = ? AND expires_at <= ?", PendingWemaStatusPending, now).
		Updates(map[string]any{"status": PendingWemaStatusExpired, "failure_reason": "alat account did not appear before expiry"}).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDuePendingWemaRegistrations returns pending rows not attempted since
// lastAttemptBefore, oldest first.
func (r *Repository) ListDuePendingWemaRegistrations(ctx context.Context, lastAttemptBefore time.Time, limit int) ([]PendingWemaRegistration, error) {
	var rows []PendingWemaRegistration
	err := r.db.WithContext(ctx).
		Where("status = ? AND (last_attempt_at IS NULL OR last_attempt_at < ?)", PendingWemaStatusPending, lastAttemptBefore).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// Encrypt and Decrypt expose the field cipher for the pending row's BVN/NIN.
func (r *Repository) Encrypt(v string) (string, error) { return r.cipher.Encrypt(v) }
func (r *Repository) Decrypt(v string) (string, error) { return r.cipher.Decrypt(v) }

// RotatePendingWemaClaimToken replaces the claim token of an active row, so
// a client resuming a registration can follow it.
func (r *Repository) RotatePendingWemaClaimToken(ctx context.Context, id, claimTokenHash string) error {
	return r.db.WithContext(ctx).Model(&PendingWemaRegistration{}).
		Where("id = ? AND status IN ?", id, []string{PendingWemaStatusPending, PendingWemaStatusProcessing}).
		Update("claim_token_hash", claimTokenHash).Error
}

// LockRegistration takes Postgres session-level advisory locks for keys (the
// phone and BVN) on a dedicated connection, held until release is called, so
// two /register calls for the same person can't both reach the provider and
// open two accounts. ok=false means another registration holds one of them.
// Locks are taken in a fixed order so two callers can't deadlock.
func (r *Repository) LockRegistration(ctx context.Context, keys ...string) (release func(), ok bool, err error) {
	sqlDB, err := r.db.DB()
	if err != nil {
		return nil, false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}

	ids := make([]int64, 0, len(keys))
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			ids = append(ids, registrationLockID(k))
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var held []int64
	release = func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		failed := false
		for _, id := range held {
			if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", id); err != nil {
				failed = true
			}
		}
		if failed {
			// A lock we couldn't release must not go back into the pool with
			// the connection: discard the connection, which ends the session
			// and frees its locks.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}

	for _, id := range ids {
		var got bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", id).Scan(&got); err != nil {
			release()
			return nil, false, err
		}
		if !got {
			release()
			return nil, false, nil
		}
		held = append(held, id)
	}
	return release, true, nil
}

// registrationLockID maps a lock key to a 64-bit advisory lock id, namespaced
// so it can't collide with other users of advisory locks.
func registrationLockID(key string) int64 {
	sum := sha256.Sum256([]byte("registerv2:" + key))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
