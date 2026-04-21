package neatsave

import (
	"context"
	"errors"
	"neat_mobile_app_backend/modules/device"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateGoal(ctx context.Context, mobileUserID, deviceID string, req CreateGoalRequest) (*CreateGoalResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("mobile user id is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("goal's name can not be empty")
	}

	targetAmount := req.TargetAmount
	if targetAmount < 50 {
		return nil, errors.New("target amount can not be less that NGN 50")
	}

	targetDate := req.TargetDate
	if targetDate.After(time.Now()) {
		return nil, errors.New("target date must be in the future")
	}

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	savingsGoals := &SavingsGoal{
		ID:           uuid.NewString(),
		MobileUserID: mobileUserID,
		Name:         name,
	}

	if err := s.repository.CreateGoal(ctx, savingsGoals); err != nil {
		return nil, errors.New("error creating a new savings goal")
	}

	return &CreateGoalResponse{
		Status:  "success",
		Message: "savings goal creation was successful",
	}, nil
}

func (s *Service) verifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	userDevice, err := s.repository.FindDevice(ctx, mobileUserID, deviceID)
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
