package neatsave

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/authchecker"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"
	"neat_mobile_app_backend/internal/middleware"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repository     *Repository
	pinVerifier    *authchecker.Verifier
	deviceVerifier DeviceVerifier
	auditLogger    auditlog.AuditLogger
}

func NewService(repository *Repository, pinVerifier *authchecker.Verifier, deviceVerifier DeviceVerifier, auditLogger auditlog.AuditLogger) *Service {
	return &Service{repository: repository, pinVerifier: pinVerifier, deviceVerifier: deviceVerifier, auditLogger: auditLogger}
}

func (s *Service) logSaveAudit(ctx context.Context, action, mobileUserID, resourceID string, status auditlog.LogStatus, metadata map[string]interface{}) {
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   resourceID,
		ResourceType: auditlog.ResourceTypeSavingsGoal,
		ActorType:    "user",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      mobileUserID,
		Action:       action,
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

func (s *Service) CreateGoal(ctx context.Context, mobileUserID string, req CreateGoalRequest) (*CreateGoalResponse, error) {
	targetAmount := req.TargetAmount
	autoSaveAmount := req.AutoSaveAmount
	name := strings.TrimSpace(req.Name)

	if targetAmount < 50 || autoSaveAmount < 50 {
		return nil, appErr.ErrInvalidSavingsAmount
	}

	targetDate := req.TargetDate
	if !targetDate.After(time.Now()) {
		return nil, appErr.ErrInvalidDateRange
	}

	preferredTime := req.PreferredTime
	if preferredTime == "" {
		preferredTime = "08:00"
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
		s.logSaveAudit(ctx, "SAVINGS_GOAL_CREATE", mobileUserID, goalID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "failed to persist savings goal",
		})
		return nil, appErr.ErrCreatingSavingsGoal
	}

	s.logSaveAudit(ctx, "SAVINGS_GOAL_CREATE", mobileUserID, goalID, auditlog.StatusSuccess, map[string]interface{}{
		"target_amount":    targetAmount,
		"auto_save_amount": autoSaveAmount,
		"auto_save":        req.AutoSave,
	})

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

func (s *Service) GetUserGoals(ctx context.Context, mobileUserID string) (*GetUserSavingsResponse, error) {
	result, err := s.repository.GetUserGoals(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrFetchingUserGoals
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

func (s *Service) GetGoalSummary(ctx context.Context, mobileUserID, goalID string) (*GetGoalSummaryResponse, error) {
	result, err := s.repository.GetGoalSummary(ctx, mobileUserID, goalID)
	if err != nil {
		return nil, appErr.ErrFetchingGoalSummary
	}

	return &GetGoalSummaryResponse{
		Status:  "success",
		Message: "Goal summary fetched successfully",
		Summary: *result,
	}, nil
}

func (s *Service) DepositFromWallet(ctx context.Context, mobileUserID, deviceID string, req DepositFromWalletRequest) (*DepositFromWalletResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("mobile user id is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		if errors.Is(err, appErr.ErrUnrecognizedDevice) {
			return nil, appErr.ErrUnrecognizedDeviceNeatsave
		}
		return nil, err
	}

	if err := s.pinVerifier.Verify(ctx, mobileUserID, req.TransactionPin); err != nil {
		return nil, err
	}

	return nil, errors.New("not implemented")
}
