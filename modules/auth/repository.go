package auth

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
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

	if err := r.db.WithContext(ctx).Table("wallet_users").Select("id,email,password,created_at").Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUserByID retrieves a user by their ID.
// It returns a pointer to models.User and an error if any.
func (r *Repository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var u models.User

	if err := r.db.WithContext(ctx).Table("wallet_users").Select("id", "email").Where("id = ?", userID).First(&u).Error; err != nil {
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

	if err := r.db.WithContext(ctx).Select("id,jti,user_id,session_id,token_hash,issued_at,expires_at").Where("jti = ?", jti).First(&token).Error; err != nil {
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
				"last_used_at":    now, // optional
			}).Error; err != nil {
			return err
		}

		if err := tx.Create(newToken).Error; err != nil {
			return err
		}

		return nil
	})
}

// func (r *Repository) RotateRefreshToken(ctx context.Context, jti string) error {
// 	return r.db.WithContext(ctx).
// }
