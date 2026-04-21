package neatsave

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
