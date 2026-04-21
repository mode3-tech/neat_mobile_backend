package neatsave

import "time"

type CreateGoalRequest struct {
	Name         string            `json:"name" binding:"required"`
	TargetAmount int64             `json:"target_amount" binding:"required"`
	TargetDate   time.Time         `json:"target_date" binding:"required"`
	SavingsType  NeatSaveMode      `json:"savings_type" binidng:"required"`
	AutoSave     bool              `json:"auto_save" binding:"required"`
	Frequency    AutoSaveFrequency `json:"frequency" binding:"frequency"`
}

type CreateGoalResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
