package neatsave

import "time"

type SavingsGoal struct {
	ID           string         `gorm:"column:id;type:text;primaryKey;index"`
	MobileUserID string         `gorm:"column:mobile_user_id;type:text;index;not null"`
	Name         string         `gorm:"column:name;type:text;not null"`
	Mode         NeatSaveMode   `gorm:"column:mode;type:text;not null;check:mode IN ('flexible','locked')"`
	TargetAmount int64          `gorm:"column:target_amount;type:bigint;not null"`
	TargetDate   time.Time      `gorm:"column:target_date;type:timestamptz;not null"`
	Status       NeatSaveStatus `gorm:"column:status;type:text;not null;check: status IN ('active','paused','completed','withdrawn')"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (SavingsGoal) TableName() string {
	return "wallet_savings_goals"
}

type AutoSaveRule struct {
	ID            string            `gorm:"column:id;type:text;primaryKey;index"`
	GoalID        string            `gorm:"column:goal_id;type:text;index;not null"`
	MobileUserID  string            `gorm:"column:mobile_user_id;type:text;index;not null"`
	Amount        int64             `gorm:"column:amount;type:bigint;not null"`
	Frequency     AutoSaveFrequency `gorm:"column:frequency;type:text;not null;check:frequency IN ('daily','weekly','monthly')"`
	PreferredTime string            `gorm:"column:preferred_time;type:time;not null"`
	NextRunDate   time.Time         `gorm:"column:next_run_date;type:date;not null"`
	IsActive      bool              `gorm:"column:is_active;not null;default:true"`
	RetryCount    int               `gorm:"column:retry_count;not null;default:0"`
	LastRunAt     *time.Time        `gorm:"column:last_run_at;type:timestamptz"`
	CreatedAt     time.Time         `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
	UpdatedAt     time.Time         `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (AutoSaveRule) TableName() string {
	return "wallet_auto_save_rules"
}

type SavingsActivity struct {
	ID           string            `gorm:"column:id;type:text;primaryKey;index"`
	GoalID       string            `gorm:"column:goal_id;type:text;index;not null"`
	MobileUserID string            `gorm:"column:mobile_user_id;type:text;index;not null"`
	EventType    NeatSaveEventType `gorm:"column:event_type;type:text;not null;check: event_type IN ('deposit','withdrawal','goal_created','goal_completed','target_reached','auto_save_triggered','auto_save_failed','goal_paused','goal_resumed')"`
	Title        string            `gorm:"column:title;type:text;not null"`
	Amount       int64             `gorm:"column:amount;type:bigint;not null"`
	Metadata     interface{}       `gorm:"column:metadata;type:jsonb;"`
	CreatedAt    time.Time         `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
}

func (SavingsActivity) TableName() string {
	return "wallet_savings_activities"
}
