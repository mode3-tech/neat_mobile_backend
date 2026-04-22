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
	autoSaveAmount := req.AutoSaveAmount

	if targetAmount < 50 || autoSaveAmount < 50 {
		return nil, errors.New("target amount and auto save amount can not be less that NGN 50")
	}

	targetDate := req.TargetDate
	if !targetDate.After(time.Now()) {
		return nil, errors.New("target date must be in the future")
	}

	preferredTime := req.PreferredTime
	if preferredTime == "" {
		preferredTime = "08:00"
	}

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	goalID := uuid.NewString()

	savingsGoal := &SavingsGoal{
		ID:           goalID,
		MobileUserID: mobileUserID,
		Name:         name,
		Mode:         req.SavingsType,
		TargetAmount: targetAmount,
		TargetDate:   targetDate,
		Status:       NeatSaveStatusActive,
	}

	var autoSaveRule *AutoSaveRule
	if req.AutoSave {
		autoSaveRule = &AutoSaveRule{
			ID:            uuid.NewString(),
			GoalID:        goalID,
			MobileUserID:  mobileUserID,
			Amount:        req.AutoSaveAmount,
			Frequency:     req.Frequency,
			PreferredTime: preferredTime,
			NextRunDate:   nextRunDate(req.Frequency),
		}
	}

	if err := s.repository.CreateGoalWithRules(ctx, savingsGoal, autoSaveRule); err != nil {
		return nil, errors.New("error creating savings goal")
	}

	return &CreateGoalResponse{
		Status:  "success",
		Message: "savings goal creation was successful",
	}, nil
}

func nextRunDate(frequency AutoSaveFrequency) time.Time {
	now := time.Now()
	switch frequency {
	case AutoSaveFrequencyWeekly:
		return now.AddDate(0, 0, 7)
	case AutoSaveFrequencyMonthly:
		return now.AddDate(0, 1, 0)
	default:
		return now.AddDate(0, 0, 1)
	}
}

func (s *Service) GetUserGoals(ctx context.Context, mobileUserID, deviceID string) (*GetUserSavingsResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("mobile user id is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	_, err := s.verifyUserDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, err
	}

	result, err := s.repository.GetUserGoals(ctx, mobileUserID)
	if err != nil {
		return nil, errors.New("error fetching user goals")
	}

	var goals []UserGoalInfo

	for _, goal := range result {
		goals = append(goals, UserGoalInfo{
			GoalID:      goal.ID,
			StartDate:   goal.CreatedAt,
			Name:        goal.Name,
			LastDeposit: goal.LastDeposit,
		})
	}

	return &GetUserSavingsResponse{
		Status:  "success",
		Message: "User's goals fetched successfully",
		Goals:   goals,
	}, nil
}

func (s *Service) GetGoalSummary(ctx context.Context, mobileUserID, deviceID, goalID string) (*GetGoalSummaryResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("mobile user id is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return nil, errors.New("goal id is required")
	}

	result, err := s.repository.GetGoalSummary(ctx, mobileUserID, goalID)
	if err != nil {
		return nil, errors.New("error fetching goal summary")
	}

	return &GetGoalSummaryResponse{
		Status:  "success",
		Message: "Goal summary fetched successfully",
		Summary: *result,
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
