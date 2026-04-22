package neatsave

import "time"

type GoalWithLastDeposit struct {
	SavingsGoal
	LastDeposit *time.Time `json:"last_deposit"`
}

type GoalSummary struct {
	Name         string     `json:"name"`
	StartDate    time.Time  `json:"start_date"`
	TargetAmount int64      `json:"target_amount"`
	TotalSavings int64      `json:"total_savings"`
	LastDeposit  *time.Time `json:"last_deposit"`
}

type NeatSaveMode string

const (
	NeatSaveModeFlexible NeatSaveMode = "flexible"
	NeatSaveModeLocked   NeatSaveMode = "locked"
)

type NeatSaveStatus string

const (
	NeatSaveStatusActive    NeatSaveStatus = "active"
	NeatSaveStatusPaused    NeatSaveStatus = "paused"
	NeatSaveStatusCompleted NeatSaveStatus = "completed"
	NeatSaveStatusWithdrawn NeatSaveStatus = "withdrawn"
)

type AutoSaveFrequency string

const (
	AutoSaveFrequencyDaily   AutoSaveFrequency = "daily"
	AutoSaveFrequencyWeekly  AutoSaveFrequency = "weekly"
	AutoSaveFrequencyMonthly AutoSaveFrequency = "monthly"
)

type NeatSaveEventType string

const (
	NeatSaveEventTypeDeposit           NeatSaveEventType = "deposit"
	NeatSaveEventTypeWithdrawal        NeatSaveEventType = "withdrawal"
	NeatSaveEventTypeGoalCreated       NeatSaveEventType = "goal_created"
	NeatSaveEventTypeTargetReached     NeatSaveEventType = "target_reached"
	NeatSaveEventTypeAutoSaveTriggered NeatSaveEventType = "auto_save_triggered"
	NeatSaveEventTypeAutoSaveFailed    NeatSaveEventType = "auto_save_failed"
	NeatSaveEventTypeGoalSaved         NeatSaveEventType = "goal_paused"
	NeatSaveEventTypeGoalResumed       NeatSaveEventType = "goal_resumed"
)
