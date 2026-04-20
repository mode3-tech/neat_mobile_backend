package auth

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository using the provided gorm DB.
// It returns a pointer to Repository.
func NewRespository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetUserByEmail retrieves a user by their email address.
// It returns a pointer to models.User and an error if any.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User

	if err := r.db.WithContext(ctx).Table("wallet_users").Select("id,email,password_hash,created_at").Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *Repository) GetUserByPhone(ctx context.Context, phone string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Table("wallet_users").Select("id,phone,password_hash,created_at").Where("phone = ?", phone).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID retrieves a user by their ID.
// It returns a pointer to models.User and an error if any.
func (r *Repository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var u models.User

	if err := r.db.WithContext(ctx).Table("wallet_users").
		Where("id = ?", userID).
		First(&u).Error; err != nil {
		return nil, err
	}

	return &u, nil
}

// AddRefreshToken adds a refresh token to the database.
// It returns an error if any.
func (r *Repository) AddRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// AddAccessToken adds an access token to the database.
// It returns an error if any.
func (r *Repository) AddAccessToken(ctx context.Context, token *models.AuthSession) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetRefreshTokenWithJTI retrieves a refresh token row from the database by its JTI.
// It returns a pointer to models.RefreshToken and an error if any.
func (r *Repository) GetRefreshTokenWithJTI(ctx context.Context, jti string) (*models.RefreshToken, error) {
	var token models.RefreshToken

	if err := r.db.WithContext(ctx).Where("jti = ?", jti).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// GetAccessTokenWithSID retrieves an access token row from the database by its SID.
// It returns a pointer to models.AuthSession and an error if any.
func (r *Repository) GetAccessTokenWithSID(ctx context.Context, sid string) (*models.AuthSession, error) {
	var token models.AuthSession
	if err := r.db.WithContext(ctx).Select("*").Where("sid = ?", sid).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteAccessToken deletes an access token row from the database by its SID.
// It returns an error if any.
func (r *Repository) DeleteAccessToken(ctx context.Context, sid string) error {
	return r.db.WithContext(ctx).Delete(&models.AuthSession{}, "sid = ?", sid).Error
}

// DeleteRefreshToken deletes a refresh token row from the database by its JTI.
// It returns an error if any.
func (r *Repository) DeleteRefreshToken(ctx context.Context, jti string) error {
	return r.db.WithContext(ctx).Delete(&models.RefreshToken{}, "jti = ?", jti).Error
}

func (r *Repository) RotateRefreshToken(ctx context.Context, oldJTI string, newToken *models.RefreshToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var oldToken models.RefreshToken

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("jti = ?", oldJTI).First(&oldToken).Error; err != nil {
			return errors.New("refresh token not found")
		}

		if oldToken.RevokedAt != nil {
			return errors.New("refresh token already revoked")
		}

		now := time.Now().UTC()
		if oldToken.ExpiresAt.Before(now) {
			return errors.New("refresh token expired")
		}

		// Revoke + link to replacement (audit)
		if err := tx.Model(&models.RefreshToken{}).
			Where("jti = ? AND revoked_at IS NULL", oldJTI).
			Updates(map[string]any{
				"revoked_at":      now,
				"replaced_by_jti": newToken.JTI,
				"last_used_at":    now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Create(newToken).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) GetValidationRow(ctx context.Context, verificationID string) (*models.VerificationRecord, error) {
	var record models.VerificationRecord
	err := r.db.WithContext(ctx).Table("wallet_verification_records").
		Select("verified_name, verified_dob, verified_phone, verified_id").
		Where("id = ? AND status = ? AND used_at = NULL", verificationID, models.VerificationStatusVerified).
		First(&record).Error

	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Repository) MarkValidationRecordUsed(ctx context.Context, verificationID string) error {
	return r.db.WithContext(ctx).Table("wallet_verification_records").
		Where("id = ?", verificationID).
		Update("used_at", time.Now().UTC()).Error
}

func (r *Repository) CreateBVNRecord(ctx context.Context, record *models.BVNRecord) error {
	if record == nil {
		return errors.New("bvn record is required")
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "bvn"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"first_name",
				"middle_name",
				"last_name",
				"gender",
				"nationality",
				"state_of_origin",
				"date_of_birth",
				"place_of_birth",
				"occupation",
				"marital_status",
				"education",
				"religion",
				"email_address",
				"passport_on_bvn",
				"passport",
				"full_home_address",
				"type_of_house",
				"city",
				"landmark",
				"living_since",
				"mobile_phone",
				"alternative_mobile_phone",
				"id_type",
				"id_number",
				"bank_name",
				"account_number",
				"next_of_kin_first_name",
				"next_of_kin_middle_name",
				"next_of_kin_last_name",
				"next_of_kin_landmark",
				"next_of_kin_phone_number",
				"next_of_kin_address",
				"next_of_kin_relationship",
				"next_of_kin_passport",
				"contact_person",
				"contact_person_phone_number",
				"customer_signature",
			}),
		}).
		Create(record).Error
}

func (r *Repository) LinkBVNRecordToUser(ctx context.Context, bvn, userID string) error {
	bvn = strings.TrimSpace(bvn)
	userID = strings.TrimSpace(userID)
	if bvn == "" || userID == "" {
		return nil
	}

	var row struct {
		ID     string `gorm:"column:id"`
		UserID string `gorm:"column:user_id"`
	}

	err := r.db.WithContext(ctx).
		Table("wallet_bvn_records").
		Select("id, user_id").
		Where("bvn = ?", bvn).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	existingUserID := strings.TrimSpace(row.UserID)
	if existingUserID != "" && existingUserID != userID {
		return errors.New("bvn already linked to another user")
	}

	return r.db.WithContext(ctx).
		Table("wallet_bvn_records").
		Where("id = ?", row.ID).
		Update("user_id", userID).Error
}

func (r *Repository) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Repository) UpdateUserPin(ctx context.Context, userID, newPinHash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("pin_hash", newPinHash).Error
}

func (r *Repository) UpdateUserPassword(ctx context.Context, userID, newPasswordHash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ? AND password_hash IS NOT NULL", userID).Update("password_hash", newPasswordHash).Error
}

func (r *Repository) UpdateCoreCustomerID(ctx context.Context, userID, coreCustomerID string) error {
	return r.db.WithContext(ctx).Model(models.User{}).Where("id = ? AND core_customer_id IS NULL", userID).Update("core_customer_id", coreCustomerID).Error
}

func (r *Repository) GetUsersWithoutCoreCustomerID(ctx context.Context, limit int) ([]PendingSyncUser, error) {
	var rows []PendingSyncUser
	err := r.db.WithContext(ctx).
		Table("wallet_users wu").
		Select("wu.id, wbr.bvn, cw.account_number, cw.account_name, cw.bank_code, cw.bank_name as bank").
		Joins("JOIN wallet_bvn_records wbr ON wbr.user_id = wu.id").
		Joins("JOIN wallet_customer_wallets cw ON cw.mobile_user_id = wu.id").
		Where("wu.core_customer_id IS NULL").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *Repository) ToggleBiometrics(ctx context.Context, mobileUserID string) (bool, error) {
	if err := r.db.WithContext(ctx).
		Model(models.User{}).
		Where("id = ?", mobileUserID).
		Update("is_biometrics_enabled", gorm.Expr("NOT is_biometrics_enabled")).Error; err != nil {
		return false, err
	}

	var user models.User
	if err := r.db.WithContext(ctx).
		Select("is_biometrics_enabled").
		Where("id = ?", mobileUserID).
		First(&user).Error; err != nil {
		return false, err
	}

	return user.IsBiometricsEnabled, nil
}
