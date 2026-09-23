package tier

import (
	"context"
	"neat_mobile_app_backend/internal/ninfacevalidator"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/nin"
)

type NINService interface {
	ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error)
}

type UserService interface {
	GetUserDetails(ctx context.Context, mobileUserID string) (*models.User, error)
	UpdateUserNIN(ctx context.Context, mobileUserID string, isNINVerified bool, ninHash string) error
}

type FaceService interface {
	ValidateNINWithFace(ctx context.Context, ninNumber, dateOfBirth, image string) (*ninfacevalidator.Result, error)
}
