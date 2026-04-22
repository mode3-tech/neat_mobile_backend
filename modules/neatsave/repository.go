package neatsave

import (
	"context"
	"neat_mobile_app_backend/modules/device"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var userDevice device.UserDevice

	if err := r.db.WithContext(ctx).
		Model(&device.UserDevice{}).
		Select("*").
		Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).
		First(&userDevice).Error; err != nil {
		return nil, err
	}

	return &userDevice, nil
}

func (r *Repository) CreateGoalWithRules(ctx context.Context, savingsGoal *SavingsGoal, rules *AutoSaveRule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(savingsGoal).Error; err != nil {
			return err
		}
		if rules != nil {
			if err := tx.Create(rules).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetUserGoals(ctx context.Context, mobileUserID string) ([]GoalWithLastDeposit, error) {
	var goals []GoalWithLastDeposit
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			g.*,
			MAX(CASE WHEN a.event_type = 'deposit' THEN a.created_at END) AS last_deposit
		FROM wallet_savings_goals g
		LEFT JOIN wallet_savings_activities a ON a.goal_id = g.id
		WHERE g.mobile_user_id = ?
		GROUP BY g.id
		ORDER BY g.created_at DESC
	`, mobileUserID).Scan(&goals).Error
	if err != nil {
		return nil, err
	}
	return goals, nil
}

func (r *Repository) GetGoalActivities(ctx context.Context, mobileUserID, goalID string) ([]SavingsActivity, error) {
	var activities []SavingsActivity
	err := r.db.WithContext(ctx).
		Where("goal_id = ? AND mobile_user_id = ?", goalID, mobileUserID).
		Order("created_at DESC").
		Find(&activities).Error
	if err != nil {
		return nil, err
	}
	return activities, nil
}

func (r *Repository) GetGoalSummary(ctx context.Context, mobileUserID, goalID string) (*GoalSummary, error) {
	var summary GoalSummary
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			g.name                                                                AS name,
			g.created_at                                                          AS start_date,
			g.target_amount                                                       AS target_amount,
			COALESCE(SUM(CASE
				WHEN a.event_type = 'deposit'    THEN a.amount
				WHEN a.event_type = 'withdrawal' THEN -a.amount
				ELSE 0
			END), 0)                                                              AS total_savings,
			MAX(CASE WHEN a.event_type = 'deposit' THEN a.created_at END)        AS last_deposit
		FROM wallet_savings_goals g
		LEFT JOIN wallet_savings_activities a ON a.goal_id = g.id
		WHERE g.id = ? AND g.mobile_user_id = ?
		GROUP BY g.id, g.name, g.created_at, g.target_amount
	`, goalID, mobileUserID).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
