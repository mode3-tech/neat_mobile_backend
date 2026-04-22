package neatsave

import "time"

type CreateGoalRequest struct {
	Name           string            `json:"name" binding:"required"`
	TargetAmount   int64             `json:"target_amount" binding:"required"`
	TargetDate     time.Time         `json:"target_date" binding:"required"`
	SavingsType    NeatSaveMode      `json:"savings_type" binding:"required"`
	AutoSave       bool              `json:"auto_save"`
	Frequency      AutoSaveFrequency `json:"frequency"`
	AutoSaveAmount int64             `json:"auto_save_amount"`
	PreferredTime  string            `json:"preferred_time"`
}

type CreateGoalResponse struct {
	Status       string     `json:"status"`
	Message      string     `json:"message"`
	StartDate    time.Time  `json:"start_date"`
	TargetAmount int64      `json:"target_amount"`
	TotalSavings int64      `json:"total_savings"`
	LastDeposit  *time.Time `json:"last_deposit"`
}

type GetUserSavingsResponse struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Goals   []UserGoalInfo `json:"goals"`
}

type UserGoalInfo struct {
	GoalID      string     `json:"goal_id"`
	Name        string     `json:"name"`
	LastDeposit *time.Time `json:"last_deposit"`
	StartDate   time.Time  `json:"start_date"`
}

type GetGoalSummaryQuery struct {
	GoalID string `form:"goal_id" binding:"required"`
}

type GetGoalSummaryResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Summary GoalSummary `json:"summary"`
}
