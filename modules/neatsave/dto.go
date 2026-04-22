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
	Status  string `json:"status"`
	Message string `json:"message"`
}
